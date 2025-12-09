package common

type ReturnStatus int

const (
	Success ReturnStatus = iota
	PlatformSpecific
	BasicGCPFailure
	BoilerPlateFailure
	FunctionalityFailure
	OtherFailure
)

func (s ReturnStatus) String() string {
	switch s {
	case Success:
		return "Success"
	case PlatformSpecific:
		return "Platform / OS Specific like read / write failures"
	case BasicGCPFailure:
		return "GCP Setup Failure"
	case BoilerPlateFailure:
		return "Basic library failure"
	case FunctionalityFailure:
		return "Functionality didn't work as expected"
	case OtherFailure:
		return "Other failure"
	}

	return "Return type undefined"
}
