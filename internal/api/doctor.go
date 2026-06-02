package api

import (
	"errors"
	"fmt"
)

const (
	DoctorPath         = "/v1/doctor"
	DoctorSourceDaemon = "daemon"
)

type DoctorResponse struct {
	Source string        `json:"source"`
	Checks []DoctorCheck `json:"checks"`
}

type DoctorCheck struct {
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func ValidateDoctorResponse(d DoctorResponse) error {
	if d.Source == "" {
		return errors.New("missing source field")
	}
	if d.Source != DoctorSourceDaemon {
		return fmt.Errorf("invalid source field %q", d.Source)
	}
	if len(d.Checks) == 0 {
		return errors.New("missing checks")
	}
	for _, check := range d.Checks {
		switch {
		case check.Name == "":
			return errors.New("missing check name")
		case !validDoctorSeverity(check.Severity):
			return errors.New("invalid check severity")
		case check.Message == "":
			return errors.New("missing check message")
		}
	}
	return nil
}

func validDoctorSeverity(severity string) bool {
	switch severity {
	case "OK", "WARN", "FAIL":
		return true
	default:
		return false
	}
}
