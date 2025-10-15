package auth

import (
  "fmt"

  "github.com/cdobbyn/azure-go-cli/pkg/config"
)

func Logout() error {
  if err := config.Delete(); err != nil {
    return err
  }

  fmt.Println("You have successfully logged out.")
  return nil
}
