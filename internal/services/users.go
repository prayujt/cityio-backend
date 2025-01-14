package services

import (
	"cityio/internal/actors"
	"cityio/internal/database"
	"cityio/internal/messages"
	"cityio/internal/models"

	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func RestoreUser(user models.User) error {
	log.Printf("Restoring user: %s", user.Username)

	userActor := actors.UserActor{}
	userPID, err := userActor.Spawn()
	if err != nil {
		log.Printf("Error spawning user actor: %s", err)
		return err
	}

	response, err := actors.Request[messages.RegisterUserResponseMessage](system.Root, userPID, messages.RegisterUserMessage{
		User:    user,
		Restore: true,
	})

	if err != nil {
		log.Printf("Error registering user: %s", err)
		return err
	}
	if response.Error != nil {
		log.Printf("Error registering user: %s", response.Error)
		return response.Error
	}

	response, err = actors.Request[messages.RegisterUserResponseMessage](system.Root, actors.GetManagerPID(), messages.AddUserPIDMessage{
		UserId: user.UserId,
		PID:    userPID,
	})

	if err != nil {
		log.Printf("Error adding user pid: %s", err)
		return err
	}
	if response.Error != nil {
		log.Printf("Error adding user pid: %s", response.Error)
		return response.Error
	}

	return nil
}

func RegisterUser(user models.RegisterUserRequest) (string, error) {
	userId := uuid.New().String()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	userActor := actors.UserActor{}
	userPID, err := userActor.Spawn()

	response, err := actors.Request[messages.RegisterUserResponseMessage](system.Root, userPID, messages.RegisterUserMessage{
		User: models.User{
			UserId:   userId,
			Username: user.Username,
			Email:    user.Email,
			Password: string(hashedPassword),
		},
		Restore: false,
	})

	if err != nil {
		log.Printf("Error registering user: %s", err)
		return "", err
	}
	if response.Error != nil {
		log.Printf("Error registering user: %s", response.Error)
		return "", response.Error
	}

	response, err = actors.Request[messages.RegisterUserResponseMessage](system.Root, actors.GetManagerPID(), messages.AddUserPIDMessage{
		UserId: userId,
		PID:    userPID,
	})

	if err != nil {
		log.Printf("Error adding user pid: %s", err)
		return "", err
	}
	if response.Error != nil {
		log.Printf("Error adding user pid: %s", response.Error)
		return "", response.Error
	}

	return userId, nil
}

func LoginUser(user models.LoginUserRequest) (models.LoginUserResponse, error) {
	db := database.GetDb()
	secretKey := []byte(os.Getenv("JWT_SECRET"))

	var account models.User
	err := db.Where("username = ?", user.Identifier).Or("email = ?", user.Identifier).First(&account).Error
	if err != nil {
		// TODO: make error message specific to login
		return models.LoginUserResponse{}, &messages.UserNotFoundError{UserId: user.Identifier}
	}

	if account.UserId == "" {
		// TODO: make error message specific to login
		return models.LoginUserResponse{}, &messages.UserNotFoundError{UserId: user.Identifier}
	}

	err = bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(user.Password))
	if err != nil {
		return models.LoginUserResponse{}, &messages.InvalidPasswordError{Identifier: user.Identifier}
	}

	claims := jwt.MapClaims{
		"username": account.Username,
		"email":    account.Email,
		"userId":   account.UserId,
		"exp":      time.Now().Add(time.Hour * 24 * 7).Unix(), // expires in a week
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(secretKey)
	if err != nil {
		return models.LoginUserResponse{}, err
	}

	return models.LoginUserResponse{
		Token:    signedToken,
		UserId:   account.UserId,
		Username: account.Username,
		Email:    account.Email,
	}, nil
}

func ValidateToken(tokenString string) (models.UserClaims, error) {
	secretKey := []byte(os.Getenv("JWT_SECRET"))
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})
	if err != nil {
		return models.UserClaims{}, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return models.UserClaims{}, &messages.InvalidTokenError{}
	}

	return models.UserClaims{
		Username: claims["username"].(string),
		Email:    claims["email"].(string),
		UserId:   claims["userId"].(string),
	}, nil
}

func GetUser(userId string) (models.User, error) {
	response, err := actors.Request[messages.GetUserPIDResponseMessage](system.Root, actors.GetManagerPID(), messages.GetUserPIDMessage{
		UserId: userId,
	})
	if err != nil {
		return models.User{}, err
	}
	if response.PID == nil {
		return models.User{}, &messages.UserNotFoundError{UserId: userId}
	}

	var userResponse *messages.GetUserResponseMessage
	userResponse, err = actors.Request[messages.GetUserResponseMessage](system.Root, response.PID, messages.GetUserMessage{})

	return userResponse.User, nil
}

func DeleteUser(userId string) error {
	response, err := actors.Request[messages.GetUserPIDResponseMessage](system.Root, actors.GetManagerPID(), messages.GetUserPIDMessage{
		UserId: userId,
	})
	if err != nil {
		log.Printf("Error getting user pid: %s", err)
		return err
	}
	if response.PID == nil {
		return &messages.UserNotFoundError{UserId: userId}
	}

	var deleteResponse *messages.DeleteUserResponseMessage
	deleteResponse, err = actors.Request[messages.DeleteUserResponseMessage](system.Root, response.PID, messages.DeleteUserMessage{})
	if err != nil {
		log.Printf("Error deleting user: %s", err)
		return err
	}
	if deleteResponse.Error != nil {
		log.Printf("Error deleting user: %s", deleteResponse.Error)
		return deleteResponse.Error
	}

	var removeResponse *messages.DeleteUserPIDResponseMessage
	removeResponse, err = actors.Request[messages.DeleteUserPIDResponseMessage](system.Root, actors.GetManagerPID(), messages.DeleteUserPIDMessage{
		UserId: userId,
	})

	if err != nil {
		log.Printf("Error removing user pid: %s", err)
		return err
	}
	if removeResponse.Error != nil {
		log.Printf("Error removing user pid: %s", removeResponse.Error)
		return removeResponse.Error
	}

	return nil
}
