package main

import (
	"github.com/aws/aws-sdk-go-v2/aws/endpoints"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3manager"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"os"
	"log"
)

var cfg aws.Config

const bucketName = "granta-cf-templates"

var bucketURL = fmt.Sprintf("https://%s.s3.us-west-2.amazonaws.com/", bucketName)

type CFTemplate struct {
	Filename  string
	StackName string
}

func collectTemplates(templates []CFTemplate) []*os.File {
	var fhs []*os.File
	for _, template := range templates {
		fh, err := os.Open(template.Filename)
		if err != nil {
			panic("unable to load cf template file")
		}
		fhs = append(fhs, fh)
	}
	return fhs
}

func uploadTemplates(cfg aws.Config, fhs []*os.File) []string {
	svc := s3manager.NewUploader(cfg)
	filenames := make([]string, 0)
	for _, fh := range fhs {
		defer fh.Close()
		output, err := svc.Upload(&s3manager.UploadInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(fh.Name()),
			Body:   fh,
		})
		if err != nil {
			panic("Failed to upload file!" + err.Error())
		}
		log.Println(output.Location)
		filenames = append(filenames, fh.Name())
	}
	return filenames
}

func loadConfig() {
	defaultcfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic("unable to load SDK config, " + err.Error())
	}
	cfg = defaultcfg
}

func wait(cfn *cloudformation.CloudFormation, stackname string, isUpdate bool) {
	var err error
	if isUpdate {
		err = cfn.WaitUntilStackUpdateComplete(&cloudformation.DescribeStacksInput{
			StackName: aws.String(stackname),
		})
	} else {
		err = cfn.WaitUntilStackCreateComplete(&cloudformation.DescribeStacksInput{
			StackName: aws.String(stackname),
		})
	}
	if err != nil {
		panic("stack update failed!")
	}
}

func launchStack(cfg aws.Config, template CFTemplate) {
	svc := cloudformation.New(cfg)

	_, err := svc.DescribeStacksRequest(&cloudformation.DescribeStacksInput{
		StackName: aws.String(template.StackName),
	}).Send()
	templateURL := bucketURL + template.Filename

	if err == nil {
		// Build the request with its input parameters
		req := svc.UpdateStackRequest(&cloudformation.UpdateStackInput{
			StackName:   aws.String(template.StackName),
			TemplateURL: aws.String(templateURL),
		})

		// Send the request, and get the response or error back
		resp, err := req.Send()
		if err != nil {
			fmt.Println("failed to update stack, " + err.Error())
		} else {
			fmt.Println("Response", resp)
			wait(svc, template.StackName, true)
		}
	} else {
		// Build the request with its input parameters
		req := svc.CreateStackRequest(&cloudformation.CreateStackInput{
			StackName:       aws.String(template.StackName),
			DisableRollback: aws.Bool(true),
			TemplateURL:     aws.String(templateURL),
		})

		// Send the request, and get the response or error back
		resp, err := req.Send()
		if err != nil {
			fmt.Println("failed to update stack, " + err.Error())
		} else {
			fmt.Println("Response", resp)
			wait(svc, template.StackName, false)
		}
	}

}

func main() {
	loadConfig()

	// Set the AWS Region that the service clients should use
	cfg.Region = endpoints.UsWest2RegionID
	cftemplates := []CFTemplate{
		{Filename: "templates/network.yaml", StackName: "granta-network"},
		{Filename: "templates/resources.yaml", StackName: "granta-resources"},
	}

	templateHandles := collectTemplates(cftemplates)
	uploadTemplates(cfg, templateHandles)

	for _, template := range cftemplates {
		launchStack(cfg, template)
	}
}
