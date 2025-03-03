package file

import (
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/src/model"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"sync"
)

type FileHandler struct{}

type Response struct {
	Mods    []service.SimpleS3Object `json:"mods"`
	Backups []service.SimpleS3Object `json:"backups"`
	Configs []service.SimpleS3Object `json:"configs"`
}

// HandleRequest Handles the request for listing files under a given prefix. Since this route is deployed
// to a lambda function and backed by the Cognito Authorizer only authorized users can invoke this.
func (f *FileHandler) HandleRequest(c *gin.Context, s3Client *service.S3Service) {
	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*model.User)

	res, err := ListAllObjects(s3Client, user.DiscordID)
	if err != nil {
		log.Errorf("failed to list s3 objects: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("failed to list s3 objects: %v", err),
		})
		return
	}
	c.JSON(http.StatusOK, res)
}

func ListAllObjects(s3Client *service.S3Service, discordId string) (*Response, error) {
	var wg sync.WaitGroup
	modsChan := make(chan []service.SimpleS3Object, 2)
	backupsChan := make(chan []service.SimpleS3Object, 1)
	configsChan := make(chan []service.SimpleS3Object, 1)
	errorChan := make(chan error, 4)

	wg.Add(4)
	go func() {
		defer wg.Done()
		objects, err := s3Client.ListObjects("mods/general/")
		if err != nil {
			errorChan <- err
			return
		}
		modsChan <- objects
	}()

	go func() {
		defer wg.Done()
		objects, err := s3Client.ListObjects(fmt.Sprintf("mods/%s/", discordId))
		if err != nil {
			errorChan <- err
			return
		}
		modsChan <- objects
	}()

	go func() {
		defer wg.Done()
		objects, err := s3Client.ListObjects(fmt.Sprintf("config/%s/", discordId))
		if err != nil {
			errorChan <- err
			return
		}
		configsChan <- objects
	}()

	go func() {
		defer wg.Done()
		objects, err := s3Client.ListObjects(fmt.Sprintf("valheim-backups-auto/%s/", discordId))
		if err != nil {
			errorChan <- err
			return
		}
		backupsChan <- objects
	}()

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(modsChan)
		close(backupsChan)
		close(configsChan)
		close(errorChan)
	}()

	if err := <-errorChan; err != nil {
		return nil, err
	}

	response := &Response{
		Mods:    make([]service.SimpleS3Object, 0),
		Backups: make([]service.SimpleS3Object, 0),
		Configs: make([]service.SimpleS3Object, 0),
	}

	for objects := range modsChan {
		response.Mods = append(response.Mods, objects...)
	}

	for objects := range configsChan {
		response.Configs = append(response.Configs, objects...)
	}

	if objects := <-backupsChan; objects != nil {
		response.Backups = objects
	}

	return response, nil
}
