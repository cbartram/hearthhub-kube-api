package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/cbartram/hearthhub-mod-api/src/util"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type GetServerHandler struct{}

type GetServerResponse struct {
	Servers []CreateServerResponse `json:"servers"`
}

func (g *GetServerHandler) HandleRequest(c *gin.Context, cognito service.CognitoService, ctx context.Context) {
	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*service.CognitoUser)

	attributes, err := cognito.GetUserAttributes(ctx, &user.Credentials.AccessToken)
	serverDetails := util.GetAttribute(attributes, "custom:server_details")
	server := CreateServerResponse{}
	if err != nil {
		log.Errorf("could not get user attributes: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("could not get user attributes: %s", err)})
		return
	}

	response := GetServerResponse{
		Servers: []CreateServerResponse{},
	}

	// If src is nil it's the first time the user is booting up.
	if serverDetails != "nil" {
		json.Unmarshal([]byte(serverDetails), &server)
		response.Servers = append(response.Servers, server)
		c.JSON(http.StatusOK, response)
		return
	}

	log.Infof("no server exists for user: %s", user.DiscordID)
	c.JSON(http.StatusOK, response)
}
