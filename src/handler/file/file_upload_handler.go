package file

import (
	"encoding/json"
	"fmt"
	"github.com/cbartram/hearthhub-common/model"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"path/filepath"
	"time"
)

var ValidExtensions = map[string]bool{
	".fwl": true,
	".db":  true,
	".zip": true,
	".cfg": true,
}

type UploadFileHandler struct{}
type FileMetadata struct {
	Name   string `json:"name"`
	Size   int    `json:"size"`
	Prefix string `json:"prefix"`
}

// HandleRequest Generates a signed url which can be used to upload a file directly to S3.
func (u *UploadFileHandler) HandleRequest(c *gin.Context, s3Client *service.S3Service, stripeService *service.StripeService) {
	bodyRaw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("could not read body from request: %s", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("could not read body from request: %v", err)})
		return
	}

	var reqBody map[string][]FileMetadata
	if err := json.Unmarshal(bodyRaw, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*model.User)
	limits, err := stripeService.GetSubscriptionLimits(user.SubscriptionId)
	if err != nil {
		log.Errorf("failed to get user subscription limits: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user subscription limits"})
		return
	}

	var urls = make(map[string]string)
	for _, file := range reqBody["files"] {
		// This is equivalent to multiplying 10 by 2^20 (2 to the power of 20)
		// Since 2^20 = 1,048,576 (approximately 1 million), this gives us 10 megabytes in bytes
		if file.Size > 30<<20 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("%s file size is too large. Maximum size is 30MB", file.Name),
			})
			return
		}

		ext := filepath.Ext(file.Name)
		if ext == "" {
			log.Errorf("no extension provide for file: %s", file.Name)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("no extension provided for file: %s", file.Name),
			})
			return
		}

		_, ok := ValidExtensions[ext]
		if !ok {
			log.Errorf("invalid extension provided for file: %s", file.Name)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("file name must end with a valid extension: *.fwl, *.db, file: %s", file.Name),
			})
			return
		}

		// Only Legend tier subscribers can upload existing worlds. This check ensures that before we generate
		// an upload url for them, they are in fact legend tier subscribers.
		if (ext == ".db" || ext == ".fwl") && !limits.ExistingWorldUpload {
			log.Errorf("user plan prohibits existing world uploads: extension %s, limits: %v", ext, limits)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "user plan prohibits existing world uploads",
			})
			return
		}

		url, err := s3Client.GeneratePutSignedUrl(file.Prefix, 45*time.Second)
		if err != nil {
			log.Errorf("failed to generated presigned url: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to generate presigned url for file: %s, err: %v", file.Name, err),
			})
			return
		}
		urls[file.Name] = url
	}

	c.JSON(http.StatusOK, gin.H{
		"urls": urls,
	})
}
