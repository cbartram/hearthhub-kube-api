package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/cbartram/hearthhub-mod-api/server/util"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

type CognitoService struct {
	cognitoClient *cognitoidentityprovider.Client
	userPoolID    string
	clientID      string
	clientSecret  string
	configPath    string
}

type CognitoCredentials struct {
	RefreshToken    string `json:"refresh_token,omitempty"`
	TokenExpiration int32  `json:"token_expiration_seconds,omitempty"`
	AccessToken     string `json:"access_token,omitempty"`
	IdToken         string `json:"id_token,omitempty"`
}

type CognitoUser struct {
	CognitoID       string             `json:"cognitoId,omitempty"`
	DiscordUsername string             `json:"discordUsername,omitempty"`
	Email           string             `json:"email,omitempty"`
	DiscordID       string             `json:"discordId,omitempty"`
	AccountEnabled  bool               `json:"accountEnabled,omitempty"`
	Credentials     CognitoCredentials `json:"credentials,omitempty"`
}

// SessionData represents locally stored session information
type SessionData struct {
	RefreshToken string `json:"refresh_token"`
}

// MakeCognitoService creates a new instance of CognitoAuthManager
func MakeCognitoService() *CognitoService {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Errorf("error loading default aws config: %s", err)
	}

	return &CognitoService{
		cognitoClient: cognitoidentityprovider.NewFromConfig(cfg),
		userPoolID:    os.Getenv("USER_POOL_ID"),
		clientID:      os.Getenv("COGNITO_CLIENT_ID"),
		clientSecret:  os.Getenv("COGNITO_CLIENT_SECRET"),
		configPath:    filepath.Join(os.Getenv("HOME"), ".config", "hearthhub-api", "session.json"),
	}
}

func (c *CognitoService) GetUserAttributes(ctx context.Context, accessToken *string) ([]types.AttributeType, error) {
	user, err := c.cognitoClient.GetUser(ctx, &cognitoidentityprovider.GetUserInput{AccessToken: accessToken})

	if err != nil {
		log.Errorf("could not get user with access token: %v", err)
		return nil, errors.New("could not get user with access token")
	}

	return user.UserAttributes, nil
}

func (c *CognitoService) UpdateUserAttributes(ctx context.Context, accessToken *string, attributes []types.AttributeType) error {
	_, err := c.cognitoClient.UpdateUserAttributes(ctx, &cognitoidentityprovider.UpdateUserAttributesInput{
		AccessToken:    accessToken,
		UserAttributes: attributes,
	})

	if err != nil {
		log.Errorf("could not update user attributes with access token: %v", err)
		return errors.New(fmt.Sprintf("could not update user attributes with access token: %v", err))
	}

	return nil
}

func (c *CognitoService) GetUser(ctx context.Context, discordId *string) (*CognitoUser, error) {
	user, err := c.cognitoClient.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(c.userPoolID),
		Username:   discordId,
	})

	if err != nil {
		log.Errorf("no user exists with username: %s, error: %v", *discordId, err)
		return nil, errors.New(fmt.Sprintf("could not get user with username: %s", *discordId))
	}

	var email, discordID, discordUsername, cognitoID string
	for _, attr := range user.UserAttributes {
		switch aws.ToString(attr.Name) {
		case "email":
			email = aws.ToString(attr.Value)
		case "sub":
			cognitoID = aws.ToString(attr.Value)
		case "custom:discord_id":
			discordID = aws.ToString(attr.Value)
		case "custom:discord_username":
			discordUsername = aws.ToString(attr.Value)
		}
	}

	// Note: This method does not return credentials with the user
	return &CognitoUser{
		DiscordUsername: discordUsername,
		DiscordID:       discordID,
		Email:           email,
		CognitoID:       cognitoID,
		AccountEnabled:  user.Enabled,
	}, nil
}

func (c *CognitoService) AuthUser(ctx context.Context, refreshToken, discordId *string) (*CognitoUser, error) {
	auth, err := c.cognitoClient.AdminInitiateAuth(ctx, &cognitoidentityprovider.AdminInitiateAuthInput{
		UserPoolId: aws.String(c.userPoolID),
		ClientId:   aws.String(c.clientID),
		AuthFlow:   types.AuthFlowTypeRefreshTokenAuth,
		AuthParameters: map[string]string{
			"REFRESH_TOKEN": *refreshToken,
			"SECRET_HASH":   util.MakeCognitoSecretHash(*discordId, c.clientID, c.clientSecret),
		},
	})

	if err != nil {
		log.Errorf("error auth: user %s could not be authenticated: %s", *discordId, err)
		return nil, err
	}

	user, err := c.cognitoClient.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(c.userPoolID),
		Username:   discordId,
	})

	if err != nil {
		log.Errorf("could not get user with username: %s: error: %s", *discordId, err.Error())
		return nil, err
	}

	var email, discordID, discordUsername, cognitoID string
	for _, attr := range user.UserAttributes {
		switch aws.ToString(attr.Name) {
		case "email":
			email = aws.ToString(attr.Value)
		case "sub":
			cognitoID = aws.ToString(attr.Value)
		case "custom:discord_id":
			discordID = aws.ToString(attr.Value)
		case "custom:discord_username":
			discordUsername = aws.ToString(attr.Value)
		}
	}

	// Note: we still authenticate a disabled user the service side handles updating UI/auth flows
	// to re-auth with discord.
	return &CognitoUser{
		DiscordUsername: discordUsername,
		DiscordID:       discordID,
		Email:           email,
		CognitoID:       cognitoID,
		AccountEnabled:  user.Enabled,
		Credentials: CognitoCredentials{
			AccessToken:     *auth.AuthenticationResult.AccessToken,
			RefreshToken:    *refreshToken,
			TokenExpiration: auth.AuthenticationResult.ExpiresIn,
			IdToken:         *auth.AuthenticationResult.IdToken,
		},
	}, nil
}
