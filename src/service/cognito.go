package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/cbartram/hearthhub-mod-api/src/util"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

type CognitoService interface {
	GetUserAttributes(ctx context.Context, accessToken *string) ([]types.AttributeType, error)
	UpdateUserAttributes(ctx context.Context, accessToken *string, attributes []types.AttributeType) error
	GetUser(ctx context.Context, discordId *string) (*CognitoUser, error)
	EnableUser(ctx context.Context, discordId string) bool
	DisableUser(ctx context.Context, discordId string) bool
	CreateCognitoUser(ctx context.Context, createUserPayload *CognitoCreateUserRequest) (*types.AuthenticationResultType, error)
	RefreshSession(ctx context.Context, discordID string) (*CognitoCredentials, error)
	AuthUser(ctx context.Context, refreshToken, userId *string) (*CognitoUser, error)
}

type CognitoServiceImpl struct {
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

// CognitoUser Defines the properties that make up a HearthHub user including, id, name, installed mods, backups
// etc... This object is returned from create user and authorize user endpoints.
type CognitoUser struct {
	CognitoID        string             `json:"cognitoId,omitempty"`
	DiscordUsername  string             `json:"discordUsername,omitempty"`
	Email            string             `json:"email,omitempty"`
	AvatarId         string             `json:"avatarId"`
	Enabled          bool               `json:"enabled"`
	DiscordID        string             `json:"discordId,omitempty"`
	InstalledMods    map[string]bool    `json:"installedMods"`
	InstalledBackups map[string]bool    `json:"installedBackups"`
	InstalledConfig  map[string]bool    `json:"installedConfig"`
	AccountEnabled   bool               `json:"accountEnabled,omitempty"`
	Credentials      CognitoCredentials `json:"credentials,omitempty"`
}

type CognitoCreateUserRequest struct {
	DiscordID       string `json:"discord_id"`
	DiscordUsername string `json:"discord_username"`
	DiscordEmail    string `json:"discord_email"`
	AvatarId        string `json:"avatar_id"`
}

// MakeCognitoService creates a new instance of CognitoAuthManager
func MakeCognitoService(awsConfig aws.Config) CognitoService {
	return &CognitoServiceImpl{
		cognitoClient: cognitoidentityprovider.NewFromConfig(awsConfig),
		userPoolID:    os.Getenv("USER_POOL_ID"),
		clientID:      os.Getenv("COGNITO_CLIENT_ID"),
		clientSecret:  os.Getenv("COGNITO_CLIENT_SECRET"),
		configPath:    filepath.Join(os.Getenv("HOME"), ".config", "hearthhub-api", "session.json"),
	}
}

func (m *CognitoServiceImpl) GetUserAttributes(ctx context.Context, accessToken *string) ([]types.AttributeType, error) {
	user, err := m.cognitoClient.GetUser(ctx, &cognitoidentityprovider.GetUserInput{AccessToken: accessToken})

	if err != nil {
		log.Errorf("could not get user with access token: %s", err.Error())
		return nil, errors.New("could not get user with access token")
	}

	return user.UserAttributes, nil
}

func (m *CognitoServiceImpl) UpdateUserAttributes(ctx context.Context, accessToken *string, attributes []types.AttributeType) error {
	_, err := m.cognitoClient.UpdateUserAttributes(ctx, &cognitoidentityprovider.UpdateUserAttributesInput{
		AccessToken:    accessToken,
		UserAttributes: attributes,
	})

	if err != nil {
		log.Errorf("could not update user attributes with access token: %s", err)
		return errors.New("could not update user attributes with access token")
	}

	return nil
}

func (m *CognitoServiceImpl) GetUser(ctx context.Context, discordId *string) (*CognitoUser, error) {
	user, err := m.cognitoClient.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(m.userPoolID),
		Username:   discordId,
	})

	if err != nil {
		log.Errorf("no user exists with username: %s, err: %v", *discordId, err)
		return nil, errors.New("could not get user with username: " + *discordId)
	}

	var email, discordID, discordUsername, cognitoID, avatarID, installedModsStr, installedBackupsStr, installedConfigStr string
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
		case "custom:avatar_id":
			avatarID = aws.ToString(attr.Value)

		// installed mods will be a json string stored when the mod is actually persisted to the pvc
		// by the hearthhub-file manager.
		case "custom:installed_mods":
			installedModsStr = aws.ToString(attr.Value)
		case "custom:installed_backups":
			installedBackupsStr = aws.ToString(attr.Value)
		case "custom:installed_config":
			installedConfigStr = aws.ToString(attr.Value)
		}
	}

	var installedMods map[string]bool
	var installedBackups map[string]bool
	var installedConfig map[string]bool
	err = json.Unmarshal([]byte(installedModsStr), &installedMods)
	err = json.Unmarshal([]byte(installedBackupsStr), &installedBackups)
	err = json.Unmarshal([]byte(installedConfigStr), &installedConfig)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to unmarshall installed file from str: %s", installedModsStr))
	}

	// Note: This method does not return credentials with the user
	return &CognitoUser{
		DiscordUsername:  discordUsername,
		DiscordID:        discordID,
		Email:            email,
		CognitoID:        cognitoID,
		AvatarId:         avatarID,
		AccountEnabled:   user.Enabled,
		InstalledMods:    installedMods,
		InstalledBackups: installedBackups,
		InstalledConfig:  installedConfig,
	}, nil
}

func (m *CognitoServiceImpl) EnableUser(ctx context.Context, discordId string) bool {
	_, err := m.cognitoClient.AdminEnableUser(ctx, &cognitoidentityprovider.AdminEnableUserInput{
		UserPoolId: aws.String(m.userPoolID),
		Username:   aws.String(discordId),
	})
	if err != nil {
		log.Errorf("failed to enable user: %s", err)
		return false
	}
	return true
}

func (m *CognitoServiceImpl) DisableUser(ctx context.Context, discordId string) bool {
	_, err := m.cognitoClient.AdminDisableUser(ctx, &cognitoidentityprovider.AdminDisableUserInput{
		UserPoolId: aws.String(m.userPoolID),
		Username:   aws.String(discordId),
	})
	if err != nil {
		log.Errorf("failed to disable user: %s", err)
		return false
	}
	return true
}

func (m *CognitoServiceImpl) CreateCognitoUser(ctx context.Context, createUserPayload *CognitoCreateUserRequest) (*types.AuthenticationResultType, error) {
	password, _ := util.MakeCrypto().GeneratePassword(util.PasswordConfig{
		Length:         15,
		RequireUpper:   true,
		RequireLower:   true,
		RequireNumber:  true,
		RequireSpecial: true,
	})

	attributes := []types.AttributeType{
		{
			Name:  aws.String("email"),
			Value: aws.String(createUserPayload.DiscordEmail),
		},
		{
			Name:  aws.String("custom:discord_id"),
			Value: aws.String(createUserPayload.DiscordID),
		},
		{
			Name:  aws.String("custom:discord_username"),
			Value: aws.String(createUserPayload.DiscordUsername),
		},
		{
			Name:  aws.String("custom:avatar_id"),
			Value: aws.String(createUserPayload.AvatarId),
		},
		{
			Name:  aws.String("custom:temporary_password"),
			Value: aws.String(password),
		},
		{
			Name:  aws.String("custom:refresh_token"),
			Value: aws.String("nil"),
		},
		{
			Name:  aws.String("custom:server_details"),
			Value: aws.String("nil"),
		},
		{
			Name:  aws.String("custom:installed_mods"),
			Value: aws.String("{}"),
		},
		{
			Name:  aws.String("custom:installed_backups"),
			Value: aws.String("{}"),
		},
		{
			Name:  aws.String("custom:installed_config"),
			Value: aws.String("{}"),
		},
	}

	_, err := m.cognitoClient.AdminCreateUser(ctx, &cognitoidentityprovider.AdminCreateUserInput{
		UserPoolId:        aws.String(m.userPoolID),
		Username:          aws.String(createUserPayload.DiscordID),
		UserAttributes:    attributes,
		MessageAction:     types.MessageActionTypeSuppress,
		TemporaryPassword: aws.String(password),
	})

	if err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	// Set permanent password although users will never actually log in with a user/pass combo. The Kraken service will use the Cognito refresh token
	// to try and get an access token for the user and authenticate with the access token.
	_, err = m.cognitoClient.AdminSetUserPassword(ctx, &cognitoidentityprovider.AdminSetUserPasswordInput{
		UserPoolId: aws.String(m.userPoolID),
		Username:   aws.String(createUserPayload.DiscordID),
		Password:   aws.String(password),
		Permanent:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("error setting permanent password: %w", err)
	}

	// Initialize auth session
	return m.initiateAuthUserPass(ctx, createUserPayload.DiscordID, password)
}

// initiateAuthUserPass Happens when a user is initially created with the user pool and uses username + generated pass to login
// The cognito refresh token and access token will be returned in the response along with the discord refresh and access
// token.
func (m *CognitoServiceImpl) initiateAuthUserPass(ctx context.Context, discordID, password string) (*types.AuthenticationResultType, error) {
	result, err := m.cognitoClient.AdminInitiateAuth(ctx, &cognitoidentityprovider.AdminInitiateAuthInput{
		UserPoolId: aws.String(m.userPoolID),
		ClientId:   aws.String(m.clientID),
		AuthFlow:   types.AuthFlowTypeAdminUserPasswordAuth,
		AuthParameters: map[string]string{
			"USERNAME":    discordID,
			"PASSWORD":    password,
			"SECRET_HASH": util.MakeCrypto().MakeCognitoSecretHash(discordID, m.clientID, m.clientSecret),
		},
	})

	if err != nil {
		return nil, fmt.Errorf("error initiating admin user/pass auth with user pool: %w", err)
	}

	// Add refresh token as custom attribute. This enables admins to get credentials on behalf of a user when
	// purchasing plugins through the Discord ticket system.
	attributes := make([]types.AttributeType, 0)
	attrName := "custom:refresh_token"
	attributes = append(attributes, types.AttributeType{
		Name:  &attrName,
		Value: result.AuthenticationResult.RefreshToken,
	})

	err = m.UpdateUserAttributes(ctx, result.AuthenticationResult.AccessToken, attributes)
	if err != nil {
		return nil, err
	}

	return result.AuthenticationResult, nil
}

// RefreshSession This method is called when a refresh token is about to expire and a new one needs to be generated.
// There is no direct way to get a new refresh token without a users password. Since we do not store the password we set
// must reset the password and re-auth to get a new refresh token.
func (m *CognitoServiceImpl) RefreshSession(ctx context.Context, discordID string) (*CognitoCredentials, error) {
	user, err := m.cognitoClient.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(m.userPoolID),
		Username:   &discordID,
	})

	if err != nil {
		log.Errorf("error: failed to get user attributes with for discord id: %s", discordID)
		return nil, errors.New(fmt.Sprintf("error: failed to get user for discord id: %s", discordID))
	}

	password := util.GetUserAttributeString(user.UserAttributes, "custom:temporary_password")

	log.Infof("auth user: %s with password", discordID)
	auth, err := m.initiateAuthUserPass(ctx, discordID, password)

	if err != nil {
		log.Errorf("error: failed to auth with user/pass for discord id: %s", discordID)
		return nil, errors.New(fmt.Sprintf("error: failed to auth with user/pass for discord id: %s", discordID))
	}

	return &CognitoCredentials{
		RefreshToken:    *auth.RefreshToken,
		TokenExpiration: auth.ExpiresIn,
		AccessToken:     *auth.AccessToken,
		IdToken:         *auth.IdToken,
	}, nil

}

func (m *CognitoServiceImpl) AuthUser(ctx context.Context, refreshToken, userId *string) (*CognitoUser, error) {
	auth, err := m.cognitoClient.AdminInitiateAuth(ctx, &cognitoidentityprovider.AdminInitiateAuthInput{
		UserPoolId: aws.String(m.userPoolID),
		ClientId:   aws.String(m.clientID),
		AuthFlow:   types.AuthFlowTypeRefreshTokenAuth,
		AuthParameters: map[string]string{
			"REFRESH_TOKEN": *refreshToken,
			"SECRET_HASH":   util.MakeCrypto().MakeCognitoSecretHash(*userId, m.clientID, m.clientSecret),
		},
	})

	if err != nil {
		log.Errorf("error auth: user %s could not be authenticated: %v", *userId, err)
		return nil, err
	}

	user, err := m.cognitoClient.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(m.userPoolID),
		Username:   userId,
	})

	if err != nil {
		log.Errorf("could not get user with username: %s: error: %s", *userId, err.Error())
		return nil, err
	}

	var email, discordID, discordUsername, cognitoID, avatarID, installedModsStr, installedBackupsStr, installedConfigStr string
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
		case "custom:avatar_id":
			avatarID = aws.ToString(attr.Value)
		case "custom:installed_mods":
			installedModsStr = aws.ToString(attr.Value)
		case "custom:installed_backups":
			installedBackupsStr = aws.ToString(attr.Value)
		case "custom:installed_config":
			installedConfigStr = aws.ToString(attr.Value)
		}
	}

	var installedMods map[string]bool
	var installedBackups map[string]bool
	var installedConfig map[string]bool
	err = json.Unmarshal([]byte(installedModsStr), &installedMods)
	err = json.Unmarshal([]byte(installedBackupsStr), &installedBackups)
	err = json.Unmarshal([]byte(installedConfigStr), &installedConfig)
	if err != nil {
		log.Errorf("failed to unmarshall installed files from str: %s", installedModsStr)
		return nil, err
	}

	// Note: we still authenticate a disabled user the service side handles updating UI/auth flows
	// to re-auth with discord.
	return &CognitoUser{
		DiscordUsername:  discordUsername,
		DiscordID:        discordID,
		Email:            email,
		CognitoID:        cognitoID,
		AccountEnabled:   user.Enabled,
		AvatarId:         avatarID,
		InstalledMods:    installedMods,
		InstalledBackups: installedBackups,
		InstalledConfig:  installedConfig,
		Credentials: CognitoCredentials{
			AccessToken:     *auth.AuthenticationResult.AccessToken,
			RefreshToken:    *refreshToken,
			TokenExpiration: auth.AuthenticationResult.ExpiresIn,
			IdToken:         *auth.AuthenticationResult.IdToken,
		},
	}, nil
}
