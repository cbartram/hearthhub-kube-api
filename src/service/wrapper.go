package service

import "gorm.io/gorm"

type Wrapper struct {
	DiscordService  *DiscordService
	S3Service       *S3Service
	StripeService   *StripeService
	CognitoService  CognitoService
	KubeService     KubernetesService
	RabbitMQService *RabbitMqService
	HearthhubDb     *gorm.DB
}
