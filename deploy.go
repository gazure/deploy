package main

import (
	"github.com/aws/aws-sdk-go-v2/aws/endpoints"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"io/ioutil"
)

func main() {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic("unable to load SDK config, " + err.Error())
	}

	// Set the AWS Region that the service clients should use
	cfg.Region = endpoints.UsWest2RegionID

	// Using the Config value, create the Cloudformation client
	svc := cloudformation.New(cfg)


	// Load CFN template
	template, err := ioutil.ReadFile("./network.yaml")
	if err != nil {
		panic("unable to read template file")
	}

	// Build the request with its input parameters
	req := svc.CreateStackRequest(&cloudformation.CreateStackInput{
		StackName: aws.String("granta-network"),
		DisableRollback: aws.Bool(true),
		TemplateBody: aws.String(string(template)),
	})

	// Send the request, and get the response or error back
	resp, err := req.Send()
	if err != nil {
		panic("failed to create stack, " + err.Error())
	}

	fmt.Println("Response", resp)
}
