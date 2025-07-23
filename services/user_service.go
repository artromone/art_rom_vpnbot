package services

import (
	"fmt"
	"log"
	"time"
	"xray-telegram-bot/database"
	"xray-telegram-bot/models"
	"xray-telegram-bot/xray"

	"github.com/google/uuid"
)

type UserService struct {
	db         *database.Database
	xrayClient *xray.Client
}

func NewUserService(db *database.Database, xrayClient *xray.Client) *UserService {
	return &UserService{
		db:         db,
		xrayClient: xrayClient,
	}
}

func (s *UserService) GetOrCreateVlessConfig(userID int64, username string) (string, string, error) {
	user, err := s.db.GetUser(userID)
	if err != nil {
		return "", "", err
	}

	if user != nil {
		vlessURL := s.xrayClient.GenerateVlessURL(user.UUID, fmt.Sprintf("user_%d", userID))
		return user.UUID, vlessURL, nil
	}

	// Create new user
	userUUID := uuid.New().String()
	email := fmt.Sprintf("user_%d@myserver", userID)

	if err := s.xrayClient.AddUser(userUUID, email); err != nil {
		return "", "", fmt.Errorf("failed to add user to Xray: %v", err)
	}

	newUser := &models.User{
		ID:        userID,
		Username:  username,
		UUID:      userUUID,
		CreatedAt: time.Now(),
	}

	if err := s.db.CreateUser(newUser); err != nil {
		// Cleanup on database error
		if removeErr := s.xrayClient.RemoveUser(email); removeErr != nil {
			log.Printf("Error cleaning up user after database insert failure: %v", removeErr)
		}
		return "", "", err
	}

	vlessURL := s.xrayClient.GenerateVlessURL(userUUID, email)
	return userUUID, vlessURL, nil
}

func (s *UserService) RemoveUser(userID int64) error {
	email := fmt.Sprintf("user_%d@myserver", userID)

	if err := s.xrayClient.RemoveUser(email); err != nil {
		log.Printf("Error removing user %d from Xray: %v", userID, err)
	}

	return s.db.DeleteUser(userID)
}

func (s *UserService) GetAllUsers() ([]*models.User, error) {
	return s.db.GetAllUsers()
}
