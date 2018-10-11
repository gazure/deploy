package main

import (
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3manager"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"os"
	"log"
	"github.com/aws/aws-sdk-go-v2/aws/endpoints"
)

var cfg aws.Config

const bucketName = "granta-cf-templates"

var bucketURL = fmt.Sprintf("https://%s.s3.us-west-2.amazonaws.com/", bucketName)

type CFTemplate struct {
	Filename  string
	StackName string
}

func (t CFTemplate) url() string {
	return bucketURL + t.Filename
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

func uploadTemplates(cfg aws.Config, templates []CFTemplate) {
	fhs := collectTemplates(templates)
	svc := s3manager.NewUploader(cfg)
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
	}
}

func loadConfig() {
	defaultcfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic("unable to load SDK config, " + err.Error())
	}
	cfg = defaultcfg
	cfg.Region = endpoints.UsWest2RegionID
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
		panic("stack update failed!" + err.Error())
	}
}

func launchStack(cfg aws.Config, template CFTemplate) {
	svc := cloudformation.New(cfg)

	_, err := svc.DescribeStacksRequest(&cloudformation.DescribeStacksInput{
		StackName: aws.String(template.StackName),
	}).Send()
	capabilities := []cloudformation.Capability{cloudformation.CapabilityCapabilityIam}

	if err == nil {
		// Build the request with its input parameters
		req := svc.UpdateStackRequest(&cloudformation.UpdateStackInput{
			StackName:    aws.String(template.StackName),
			TemplateURL:  aws.String(template.url()),
			Capabilities: capabilities,
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
			TemplateURL:     aws.String(template.url()),
			Capabilities:    capabilities,
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

func validate(template CFTemplate) error {
	cfn := cloudformation.New(cfg)
	req := cfn.ValidateTemplateRequest(&cloudformation.ValidateTemplateInput{
		TemplateURL: aws.String(template.url()),
	})
	_, err := req.Send()
	return err
}

func validateTemplates(templates []CFTemplate) {
	errs := make([]error, len(templates))
	hasError := false
	for i, tmpl := range templates {
		errs[i] = validate(tmpl)
		if errs[i] != nil {
			fmt.Println(tmpl.Filename + ": " + errs[i].Error())
			hasError = true
		}
	}
	if hasError {
		panic("Template validation error")
	} else {
		fmt.Println("No validation errors")
	}
}

func launchStacks(templates []CFTemplate) {
	for _, template := range templates {
		launchStack(cfg, template)
	}
}

func main() {
	loadConfig()

	cftemplates := []CFTemplate{
		{Filename: "templates/network.yaml", StackName: "granta-network"},
		{Filename: "templates/resources.yaml", StackName: "granta-resources"},
		{Filename: "templates/cluster.yaml", StackName: "granta-cluster"},
		{Filename: "templates/service.yaml", StackName: "granta-oauth-service"},
	}

	uploadTemplates(cfg, cftemplates)
	validateTemplates(cftemplates)
	launchStacks(cftemplates)
}
