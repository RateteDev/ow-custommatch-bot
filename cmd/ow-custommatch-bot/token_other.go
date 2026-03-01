//go:build !windows

package main

func readTokenFromStore() (string, error) {
	return "", errTokenStoreUnsupported
}

func saveTokenToStore(string) error {
	return errTokenStoreUnsupported
}

func deleteTokenFromStore() error {
	return errTokenStoreUnsupported
}
