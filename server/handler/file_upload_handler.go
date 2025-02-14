package handler

import (
	"context"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"path/filepath"
	"strings"
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

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("no file provided: %v", err),
		})
		return
	}
	defer file.Close()

	prefix := c.Query("query")

	// This is equivalent to multiplying 10 by 2^20 (2 to the power of 20)
	// Since 2^20 = 1,048,576 (approximately 1 million), this gives us 10 megabytes in bytes
	if header.Size > 30<<20 {
		log.Errorf("file size too large")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "file too large. Maximum size is 30MB",
		})
		return
	}

	// Validate file extension
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		log.Errorf("no extension provided: %s", header.Filename)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "file name must end with a valid extension: *.fwl, *.db, *.zip, *.cfg",
		})
		return
	}

	ext = ext[1:]
	_, ok := ValidExtensions[ext]
	if !ok {
		log.Errorf("invalid extension: %s", ext)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid extension: %s", ext),
		})
		return
	}

	_, ok = ValidPrefixes[prefix]
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

	path := fmt.Sprintf("%s/%s/%s", sanitizedPrefix, user.DiscordID, header.Filename)

	_, err = s3Client.UploadObject(context.Background(), path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to upload file: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("file upload ok: %s", path),
	})
}
