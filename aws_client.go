package main

import (
	"context"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var ec2Client *ec2.Client

// InitAWS loads the AWS configuration and creates the EC2 client
func InitAWS() {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(AWSRegion))
	if err != nil {
		log.Fatalf("Unable to load SDK config: %v", err)
	}
	ec2Client = ec2.NewFromConfig(cfg)
}

// getInstances returns IPs of running instances matching tags and target OS
func getInstances(keys []string, values []string, targetOS string) ([]string, error) {
	var ips []string

	// 1. AWS Filter: Get ONLY running instances matching user tags
	filters := []types.Filter{
		{Name: aws.String("instance-state-name"), Values: []string{"running"}},
	}

	for i, k := range keys {
		filters = append(filters, types.Filter{
			Name:   aws.String("tag:" + k),
			Values: []string{values[i]},
		})
	}

	input := &ec2.DescribeInstancesInput{
		Filters: filters,
	}

	paginator := ec2.NewDescribeInstancesPaginator(ec2Client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return nil, err
		}

		for _, res := range page.Reservations {
			for _, inst := range res.Instances {

				// --- 2. OS PLATFORM CHECK (FIXED) ---

				// inst.Platform is of type 'types.PlatformValues' (string alias).
				// It is NOT a pointer, so we compare it directly.
				// "windows" = Windows
				// "" (Empty string) = Linux/Unix

				isWindows := false
				// Cast to string to compare safely
				if strings.ToLower(string(inst.Platform)) == "windows" {
					isWindows = true
				}

				// Filtering Logic
				if targetOS == "windows" && !isWindows {
					continue // Skip: User wants Windows, but Instance is Linux
				}
				if targetOS == "linux" && isWindows {
					continue // Skip: User wants Linux, but Instance is Windows
				}

				// --- 3. Collect IP ---
				if inst.PrivateIpAddress != nil {
					ips = append(ips, *inst.PrivateIpAddress)
				} else if inst.PublicIpAddress != nil {
					ips = append(ips, *inst.PublicIpAddress)
				}
			}
		}
	}
	return ips, nil
}
