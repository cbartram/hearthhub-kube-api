package handler

import (
	"context"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"path/filepath"
)

var ValidExtensions = map[string]bool{
	"fwl": true,
	"db":  true,
	"zip": true,
	"cfg": true,
}

var ValidPrefixes = map[string]bool{
	"backups":  true,
	"configs":  true,
	"mods":     true,
	"backups/": true,
	"configs/": true,
	"mods/":    true,
}

type UploadFileHandler struct{}

// HandleRequest handles file uploads to S3
func (u *UploadFileHandler) HandleRequest(c *gin.Context, s3Client *service.S3Service) {
	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*service.CognitoUser)

	log.Infof("processing files")

	seedFile, seedHeader, err := c.Request.FormFile("seed_file")

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No seed file provided"})
		return
	}
	log.Infof("seed header: %s", seedHeader.Filename)
	defer seedFile.Close()

	worldFile, worldHeader, err := c.Request.FormFile("world_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No world file provided"})
		return
	}
	defer worldFile.Close()
	log.Infof("world header: %s", worldHeader.Filename)
	log.Infof("got files!")

	// This is equivalent to multiplying 10 by 2^20 (2 to the power of 20)
	// Since 2^20 = 1,048,576 (approximately 1 million), this gives us 10 megabytes in bytes
	if seedHeader.Size > 30<<20 || worldHeader.Size > 30<<20 {
		log.Errorf("file size of db or fwl too large")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "one of the files sizes is too large. Maximum size is 30MB",
		})
		return
	}

	// Validate file extension
	worldExt := filepath.Ext(worldHeader.Filename)
	seedExt := filepath.Ext(seedHeader.Filename)
	if worldExt == "" || seedExt == "" {
		log.Errorf("no extension provided: world: %s, seed: %s", worldHeader.Filename, seedHeader.Filename)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "file name must end with a valid extension: *.fwl, *.db",
		})
		return
	}

	worldExt = worldExt[1:]
	seedExt = seedExt[1:]
	_, ok := ValidExtensions[worldExt]
	_, okSeeds := ValidExtensions[seedExt]
	if !ok || !okSeeds {
		log.Errorf("invalid extension: world: %s, seed: %s", worldExt, seedExt)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid extension: world: %s, seed: %s", worldExt, seedExt),
		})
		return
	}

	worldPath := fmt.Sprintf("valheim-backups-auto/%s/%s", user.DiscordID, worldHeader.Filename)
	seedPath := fmt.Sprintf("valheim-backups-auto/%s/%s", user.DiscordID, seedHeader.Filename)

	// TODO Handling if 1 of these fails?
	_, err = s3Client.UploadObject(context.Background(), worldPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to upload world file: %v", err),
		})
		return
	}

	_, err = s3Client.UploadObject(context.Background(), seedPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to upload seed file: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "file upload ok",
		"files":   []string{worldPath, seedPath},
	})
}
