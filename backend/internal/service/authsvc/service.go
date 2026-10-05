package authsvc

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"sn-backend/internal/model"
	"sn-backend/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailTaken         = errors.New("auth: email already registered")
	ErrNicknameTaken      = errors.New("auth: nickname already taken")
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrInvalidInput       = errors.New("auth: invalid registration input")
)

// Same limits as the frontend (frontend/lib/validate.js).
const (
	maxNameLen    = 50
	maxEmailLen   = 254
	maxAboutMeLen = 500
	minPassword   = 8
	maxPassword   = 72 // bcrypt only reads the first 72 bytes
)

// dummyHash is compared against when the account does not exist, so a failed
// login takes as long whether or not the email is registered.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy password"), bcrypt.DefaultCost)

type Service struct{ users *repository.UserRepository }

func New(users *repository.UserRepository) *Service { return &Service{users: users} }

func (s *Service) UserByID(id int64) (*model.User, error) { return s.users.GetUserByID(id) }

func (s *Service) Login(identifier, password string) (*model.User, error) {
	var user *model.User
	var err error

	identifier = strings.ToLower(strings.TrimSpace(identifier))
	if isValidEmail(identifier) {
		user, err = s.users.GetUserByEmail(identifier)
	} else if isValidNickname(identifier) {
		user, err = s.users.GetUserByNickname(identifier)
	} else {
		return nil, ErrInvalidCredentials
	}

	if err != nil || user == nil {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return nil, ErrInvalidCredentials
	}

	if bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(password),
	) != nil {
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

// Register checks the new user, hashes its plain password and stores it.
func (s *Service) Register(user *model.User) error {
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	user.Nickname = strings.ToLower(strings.TrimSpace(user.Nickname))
	user.FirstName = strings.TrimSpace(user.FirstName)
	user.LastName = strings.TrimSpace(user.LastName)
	user.AboutMe = strings.TrimSpace(user.AboutMe)
	if user.Nickname == "" {
		nickname, err := s.newNickname(user.FirstName, user.LastName)
		if err != nil {
			return err
		}
		user.Nickname = nickname
	}
	if err := validateRegisterInput(user); err != nil {
		return err
	}
	if _, err := s.users.GetUserByEmail(user.Email); err == nil {
		return ErrEmailTaken
	} else if !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	// login accepts a nickname too, so two accounts cannot share one
	if _, err := s.users.GetUserByNickname(user.Nickname); err == nil {
		return ErrNicknameTaken
	} else if !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hash)
	return s.users.CreateUser(user)
}

// newNickname builds a free nickname from the first and last name, like
// "johnsmith", then "johnsmith2", "johnsmith3"... when it is already taken.
func (s *Service) newNickname(firstName, lastName string) (string, error) {
	base := ""
	for _, r := range strings.ToLower(firstName + lastName) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			base += string(r)
		}
	}
	// a nickname needs 4 characters and a letter (names can be "Li" or "José")
	if len(base) < 4 || !letterRegex.MatchString(base) {
		base = "user" + base
	}
	for number := 1; ; number++ {
		suffix := ""
		if number > 1 {
			suffix = strconv.Itoa(number)
		}
		// keep the whole nickname within 15 characters
		nickname := base[:min(len(base), 15-len(suffix))] + suffix
		_, err := s.users.GetUserByNickname(nickname)
		if errors.Is(err, repository.ErrNotFound) {
			return nickname, nil
		}
		if err != nil {
			return "", err
		}
	}
}

func validateRegisterInput(input *model.User) error {
	if strings.TrimSpace(input.Email) == "" {
		return fmt.Errorf("%w: email is required", ErrInvalidInput)
	}
	if len(input.Email) > maxEmailLen || !isValidEmail(input.Email) {
		return fmt.Errorf("%w: invalid email address", ErrInvalidInput)
	}

	if strings.TrimSpace(input.Nickname) == "" {
		return fmt.Errorf("%w: nickname is required", ErrInvalidInput)
	}
	if !isValidNickname(input.Nickname) {
		return fmt.Errorf("%w: invalid nickname", ErrInvalidInput)
	}

	if strings.TrimSpace(input.Password) == "" {
		return fmt.Errorf("%w: password is required", ErrInvalidInput)
	}
	if len(input.Password) < minPassword || len(input.Password) > maxPassword {
		return fmt.Errorf("%w: password must be %d to %d characters", ErrInvalidInput, minPassword, maxPassword)
	}

	if input.FirstName == "" || input.LastName == "" {
		return fmt.Errorf("%w: first and last name are required", ErrInvalidInput)
	}
	if utf8.RuneCountInString(input.FirstName) > maxNameLen || utf8.RuneCountInString(input.LastName) > maxNameLen {
		return fmt.Errorf("%w: names must be %d characters or less", ErrInvalidInput, maxNameLen)
	}
	if utf8.RuneCountInString(input.AboutMe) > maxAboutMeLen {
		return fmt.Errorf("%w: about me must be %d characters or less", ErrInvalidInput, maxAboutMeLen)
	}

	birthDate, err := time.Parse("2006-01-02", input.DateOfBirth)
	if err != nil {
		return fmt.Errorf("%w: date of birth must be YYYY-MM-DD", ErrInvalidInput)
	}

	today := time.Now()

	if birthDate.After(today) {
		return fmt.Errorf("%w: date of birth cannot be in the future", ErrInvalidInput)
	}

	age := today.Year() - birthDate.Year()

	if today.Month() < birthDate.Month() ||
		(today.Month() == birthDate.Month() && today.Day() < birthDate.Day()) {
		age--
	}

	if age < 13 {
		return fmt.Errorf("%w: must be at least 13 years old", ErrInvalidInput)
	}

	return nil
}

var (
	emailRegex    = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
	nicknameRegex = regexp.MustCompile(`^[a-z0-9]{4,15}$`)
	letterRegex   = regexp.MustCompile(`[a-z]`)
)

func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func isValidNickname(nickname string) bool {
	return nicknameRegex.MatchString(nickname) &&
		letterRegex.MatchString(nickname)
}
