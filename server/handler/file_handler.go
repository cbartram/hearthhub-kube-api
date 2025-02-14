package handler

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/cbartram/hearthhub-mod-api/server/util"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"slices"
	"strings"
)

type FileHandler struct{}

// HandleRequest Handles the request for listing files under a given prefix. Since this route is deployed
// to a lambda function and backed by the Cognito Authorizer only authorized users can invoke this.
func (f *FileHandler) HandleRequest(c *gin.Context, s3Client *service.S3Service) {
	prefix := c.Query("prefix")

	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*service.CognitoUser)

	// valid prefixes are stored in file_upload_handler.go and essentially are just:
	// config, backups, mods to direct the s3 operation at where to list or put user files
	_, ok := ValidPrefixes[prefix]
	if !ok {
		log.Errorf("invalid prefix: %s", prefix)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid prefix: %s", prefix),
		})
		return
	}

	var sanitizedPrefix = prefix
	if strings.HasSuffix("/", prefix) {
		sanitizedPrefix = prefix[0 : len(prefix)-1]
	}

	path := fmt.Sprintf("%s/%s/", sanitizedPrefix, user.DiscordID)
	log.Infof("prefix is sanitized and valid: %s, listing objects for path: %s", sanitizedPrefix, path)

	objs, err := s3Client.ListObjects(path)
	if err != nil {
		log.Errorf("failed to list objects: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to list objects: %v", err),
		})
		return
	}

	// Also perform a list objects on the default mods available  and concat the lists
	if prefix == "mods" {
		log.Infof("prefix is: mods, fetching default mods as well as custom mods for user: %s", user.DiscordID)
		defaultObjs, err := s3Client.ListObjects("mods/general/")
		if err != nil {
			log.Errorf("failed to list default mods: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to list default mods: %v", err),
			})
			return
		}
		objs = slices.Concat(objs, defaultObjs)
	}

	if prefix == "backups" {
		log.Infof("prefix is: backups fetching auto backups as well as uploaded backups")
		autoBackups, err := s3Client.ListObjects(fmt.Sprintf("valheim-backups-auto/%s/", user.DiscordID))
		if err != nil {
			log.Errorf("failed to list auto backups: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to list auto backups: %v", err),
			})
			return
		}
		objs = slices.Concat(objs, autoBackups)
	}

	// Map the s3 objects into a simpler form with just the key and size (additional attr can be added later)
	// if needed
	simpleObjs := util.Map[types.Object, service.SimpleS3Object](objs, func(o types.Object) service.SimpleS3Object {
		return service.SimpleS3Object{
			Key:  *o.Key,
			Size: *o.Size,
		}
	})

	c.JSON(http.StatusOK, gin.H{
		"files": simpleObjs,
	})
}
