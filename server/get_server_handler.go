package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/cbartram/hearthhub-mod-api/server/util"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"strconv"
)

type GetServerHandler struct{}

type GetServerResponse struct {
	Servers     []CreateServerResponse `json:"servers"`
	CpuLimit    int                    `json:"cpu_limit"`
	MemoryLimit int                    `json:"memory_limit"`
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

	cpuLimit, _ := strconv.Atoi(os.Getenv("CPU_LIMIT"))
	memLimit, _ := strconv.Atoi(os.Getenv("MEMORY_LIMIT"))

	response := GetServerResponse{
		Servers:     []CreateServerResponse{},
		CpuLimit:    cpuLimit,
		MemoryLimit: memLimit,
	}

	// If server is nil it's the first time the user is booting up.
	if serverDetails != "nil" {
		json.Unmarshal([]byte(serverDetails), &server)
		response.Servers = append(response.Servers, server)
		c.JSON(http.StatusOK, response)
		return
	}

	log.Infof("no server exists for user: %s", user.DiscordID)
	c.JSON(http.StatusOK, response)
}
