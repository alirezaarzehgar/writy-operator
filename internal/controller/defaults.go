package controller

import "fmt"

var (
	defaultStorageClaimName        = "data"
	defaultStorageClassName        = "standard"
	writyImage                     = "alirezaarzehgar/writy"
	writyBinaryPath                = "/bin/writy"
	defaultWrityImageVersion       = "v1.0.0"
	defaultWrityPort         int32 = 8000
	defaultLoadbalancerPort  int32 = 3000
	defaultLogLevel                = "warn"
)

func getServiceName(wcName string) string {
	return fmt.Sprintf("%s-service", wcName)
}

func getBalancerName(wcName string) string {
	return fmt.Sprintf("%sloadbalancer", wcName)
}
