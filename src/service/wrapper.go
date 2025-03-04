package service

import (
	"github.com/cbartram/hearthhub-common/service"
	"gorm.io/gorm"
)

type Wrapper struct {
	DiscordService  *DiscordService
	S3Service       *S3Service
	StripeService   *StripeService
	CognitoService  service.CognitoService
	KubeService     KubernetesService
	RabbitMQService *RabbitMqService
	HearthhubDb     *gorm.DB
}
