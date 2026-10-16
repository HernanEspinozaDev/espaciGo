package main

import "errors"

func authPrototypeEnabled(getenv func(string) string) (bool, error) {
	switch getenv("APP_PROFILE") {
	case "":
		return getenv("LOCAL_AUTH_PROTOTYPE") == "1", nil
	case "cloud-test":
		if getenv("LOCAL_AUTH_PROTOTYPE") != "" {
			return false, errors.New("cloud-test profile cannot combine with LOCAL_AUTH_PROTOTYPE")
		}
		return true, nil
	default:
		return false, errors.New("APP_PROFILE must be cloud-test when configured")
	}
}
