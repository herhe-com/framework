package auth

import (
	"fmt"
	"github.com/herhe-com/framework/contracts/auth"
	"golang.org/x/crypto/bcrypt"
	"strconv"
	"strings"
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

func NameOfRoleWithDeveloper() string {
	return NameOfRole(auth.CodeOfDeveloper)
}

func NameOfRoleWithPlatform() string {
	return NameOfRole(auth.CodeOfPlatform)
}

func NameOfRoleWithClique() string {
	return NameOfRole(auth.CodeOfClique)
}

func NameOfRoleWithRegion() string {
	return NameOfRole(auth.CodeOfRegion)
}

func NameOfRoleWithStore() string {
	return NameOfRole(auth.CodeOfStore)
}

func Password(password string) string {

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	return string(hash)
}

func CheckPassword(password, crypt string) bool {

	err := bcrypt.CompareHashAndPassword([]byte(crypt), []byte(password))

	return err == nil
}
