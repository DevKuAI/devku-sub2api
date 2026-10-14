package bi

import "golang.org/x/mod/semver"

func requireClientVersion(current, minimum string) error {
	if minimum == "" {
		minimum = "0.2.0"
	}
	if !semver.IsValid("v" + current) {
		return invalid("client_version", "Expected a semantic version")
	}
	if semver.Compare("v"+current, "v"+minimum) < 0 {
		err := apiError(426, "CLIENT_UPGRADE_REQUIRED", "Upgrade the mini-program before signing in")
		err.MinClientVersion = minimum
		return err
	}
	return nil
}
