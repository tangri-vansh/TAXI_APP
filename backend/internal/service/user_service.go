package service

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"

	"safe-ride/internal/model"
	"safe-ride/internal/repository"
)

var ErrInvalidInput = errors.New("invalid input")

type UserService struct {
	userRepository *repository.UserRepository
}

func (s *UserService) CreateUser(
	ctx context.Context,
	name string,
	phone string,
	password string,
) (*model.User, error) {

	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}

	if phone == "" {
		return nil, fmt.Errorf("%w: phone is required", ErrInvalidInput)
	}

	if password == "" {
		return nil, fmt.Errorf("%w: password is required", ErrInvalidInput)
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Name:         name,
		Phone:        phone,
		PasswordHash: string(passwordHash),
	}

	err = s.userRepository.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func NewUserService(userRepository *repository.UserRepository) *UserService {
	return &UserService{
		userRepository: userRepository,
	}
}
