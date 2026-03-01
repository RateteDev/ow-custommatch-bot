//go:build windows

package main

import (
	"errors"
	"strings"

	"github.com/danieljoos/wincred"
)

func readTokenFromStore() (string, error) {
	cred, err := wincred.GetGenericCredential(tokenStoreTarget)
	if err != nil {
		if errors.Is(err, wincred.ErrElementNotFound) {
			return "", errTokenNotFound
		}
		return "", err
	}

	token := strings.TrimSpace(string(cred.CredentialBlob))
	if token == "" {
		return "", errTokenNotFound
	}
	return token, nil
}

func saveTokenToStore(token string) error {
	cred := wincred.NewGenericCredential(tokenStoreTarget)
	cred.CredentialBlob = []byte(strings.TrimSpace(token))
	cred.Comment = appName + " Discord bot token"
	return cred.Write()
}

func deleteTokenFromStore() error {
	cred, err := wincred.GetGenericCredential(tokenStoreTarget)
	if err != nil {
		if errors.Is(err, wincred.ErrElementNotFound) {
			return errTokenNotFound
		}
		return err
	}
	return cred.Delete()
}
