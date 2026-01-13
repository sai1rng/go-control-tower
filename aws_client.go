package main

import (
	"context"
	"log"

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

// getInstances returns IPs of running instances matching ALL provided tag pairs
func getInstances(keys []string, values []string) ([]string, error) {
	var ips []string

	// Base filter: Instance must be running
	filters := []types.Filter{
		{Name: aws.String("instance-state-name"), Values: []string{"running"}},
	}

	// Append a filter for every Key/Value pair provided
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
				// Prefer Private IP, fallback to Public IP
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
