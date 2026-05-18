package auth

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/herhe-com/framework/facades"
	"github.com/samber/lo"
	"golang.org/x/crypto/bcrypt"
)

func Name(args ...any) string {

	names := make([]string, 0)

	for _, item := range args {
		names = append(names, fmt.Sprintf("%v", item))
	}

	return strings.Join(names, ":")
}

func NameOfUser(args ...any) string {
	return Name(append([]any{"USER"}, args...)...)
}

func NameOfRole(args ...any) string {
	return Name(append([]any{"ROLE"}, args...)...)
}

func NameOfPermission(platform uint16, id *string, permission string) (permissions []string) {
	permissions = append(permissions, strconv.Itoa(int(platform)))
	if id != nil {
		permissions = append(permissions, *id)
	}
	permissions = append(permissions, permission)
	return permissions
}

func NameOfDeveloper() string {
	return NameOfRole(CodeOfDeveloper)
}

func NameOfPlatform() string {
	return NameOfRole(CodeOfPlatform)
}

func NameOfClique() string {
	return NameOfRole(CodeOfClique)
}

func NameOfRegion() string {
	return NameOfRole(CodeOfRegion)
}

func NameOfStore() string {
	return NameOfRole(CodeOfStore)
}

func Password(password string) string {

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	return string(hash)
}

func CheckPassword(password, crypt string) bool {

	err := bcrypt.CompareHashAndPassword([]byte(crypt), []byte(password))

	return err == nil
}

func DefaultPlatform() uint16 {

	platforms, ok := facades.Config().Get("auth.platforms").([]uint16)
	if !ok || len(platforms) == 0 {
		return 0
	}

	return lo.Min(platforms)
}
