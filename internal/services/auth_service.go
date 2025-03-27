package services

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/kimutaiwycliff/auth-service/internal/models"
	"github.com/kimutaiwycliff/auth-service/internal/repositories"
	"github.com/kimutaiwycliff/auth-service/internal/utils"
	"github.com/redis/go-redis/v9"
)

type AuthService interface {
	Register(email, password string) (*models.User, error)
	Login(email, password string) (*models.TokenPair, error)
	RefreshToken(ctx context.Context,refreshToken string) (*models.TokenPair, error)
	Logout(userID, accessToken string) error
	VerifyToken(token string) (string, error) // returns userID
	GetUser(userID string) (*models.User, error)
}

type authService struct {
	userRepo    repositories.UserRepository
	jwtService  JWTService
	redisService RedisService
}

func NewAuthService(
	userRepo repositories.UserRepository,
	jwtService JWTService,
	redisService RedisService,
) AuthService {
	return &authService{
		userRepo:    userRepo,
		jwtService:  jwtService,
		redisService: redisService,
	}
}

func (s *authService) Register(email, password string) (*models.User, error) {
	// Validate input
	if !utils.IsEmailValid(email) {
		return nil, errors.New("invalid email format")
	}

	if !utils.IsPasswordValid(password) {
		return nil, errors.New("password must be at least 8 characters")
	}

	// Check if user exists
	existingUser, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return nil, err
	}
	if existingUser != nil {
		return nil, errors.New("user already exists")
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// Create user
	user := &models.User{
		Email:    email,
		Password: hashedPassword,
	}

	return s.userRepo.Create(user)
}

func (s *authService) Login(email, password string) (*models.TokenPair, error) {
	// Find user
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("invalid credentials")
	}

	// Verify password
	if !utils.VerifyPassword(password, user.Password) {
		return nil, errors.New("invalid credentials")
	}

	// Generate tokens
	accessToken, err := s.jwtService.GenerateAccessToken(user.ID)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, err
	}

	// Store refresh token using RedisService
	err = s.redisService.StoreRefreshToken(context.Background(), user.ID, refreshToken, s.jwtService.GetRefreshExpiry())
	if err != nil {
		return nil, err
	}

	return &models.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*models.TokenPair, error) {
	// Validate refresh token structure and signature
	claims, err := s.jwtService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Type assertion for userID with proper error handling
	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return nil, errors.New("invalid user ID in token claims")
	}

	// Verify refresh token exists in Redis and matches
	storedToken, err := s.redisService.GetRefreshToken(ctx, userID)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errors.New("refresh token not found or expired")
		}
		return nil, fmt.Errorf("failed to verify refresh token: %w", err)
	}

	if storedToken != refreshToken {
		// Potential security issue - log this event
		s.redisService.DeleteRefreshToken(ctx, userID) // Invalidate compromised token
		return nil, errors.New("refresh token mismatch - possible token reuse")
	}

	// Generate new tokens
	newAccessToken, err := s.jwtService.GenerateAccessToken(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err := s.jwtService.GenerateRefreshToken(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Atomic operation: Delete old and store new refresh token
	err = s.redisService.StoreRefreshToken(ctx, userID, newRefreshToken, s.jwtService.GetRefreshExpiry())
	if err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Invalidate the old refresh token (optional additional security)
	if err := s.redisService.BlacklistToken(ctx, refreshToken, s.jwtService.GetRefreshExpiry()); err != nil {
		// Log this error but don't fail the operation
		log.Printf("WARNING: failed to blacklist old refresh token: %v", err)
	}

	return &models.TokenPair{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (s *authService) Logout(userID, accessToken string) error {
	// Blacklist access token
	err := s.redisService.BlacklistToken(context.Background(), accessToken, s.jwtService.GetAccessExpiry())
	if err != nil {
		return err
	}

	// Delete refresh token
	return s.redisService.DeleteRefreshToken(context.Background(), userID)
}

func (s *authService) VerifyToken(token string) (string, error) {
	// Check if token is blacklisted
	blacklisted, err := s.redisService.IsTokenBlacklisted(context.Background(), token)
	if err != nil {
		return "", err
	}
	if blacklisted {
		return "", errors.New("token is invalidated")
	}

	// Validate token
	claims, err := s.jwtService.ValidateAccessToken(token)
	if err != nil {
		return "", err
	}

	return claims["sub"].(string), nil
}

func (s *authService) GetUser(userID string) (*models.User, error) {
	return s.userRepo.FindByID(userID)
}
