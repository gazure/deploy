 package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfnTypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"os"
	"log"
)

var cfg aws.Config
var ctx context.Context

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
	svc := manager.NewUploader(s3.NewFromConfig(cfg))
	defer func() {
		fhs := fhs
		for _, fh := range fhs {
			err := fh.Close()
			if err != nil {
				log.Println("Error closing file handle")
			}
		}
	}()
	for _, fh := range fhs {
		output, err := svc.Upload(ctx, &s3.PutObjectInput{
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
	defaultcfg, err := config.LoadDefaultConfig(ctx, config.WithDefaultRegion("us-west-2"))
	if err != nil {
		panic("unable to load SDK config, " + err.Error())
	}
	cfg = defaultcfg
	cfg.Region = "us-west-2"
}

func wait(cfn *cloudformation.Client, stackName string, isUpdate bool) {
	var err error
	if isUpdate {
		waiter := cloudformation.TypeRegistrationCompleteWaiter{}()
		err = cfn.WaitUntilStackUpdateComplete(&cloudformation.DescribeStacksInput{
			StackName: aws.String(stackName),
		})
	} else {
		err = cfn.WaitUntilStackCreateComplete(&cloudformation.DescribeStacksInput{
			StackName: aws.String(stackName),
		})
	}
	if err != nil {
		panic("stack update failed!" + err.Error())
	}
}

func launchStack(cfg aws.Config, template CFTemplate) {
	svc := cloudformation.NewFromConfig(cfg)

	_, err := svc.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(template.StackName),
	})
	capabilities := []cfnTypes.Capability{cfnTypes.CapabilityCapabilityIam}

	if err == nil {
		// Build the request with its input parameters
		resp, err := svc.UpdateStack(ctx, &cloudformation.UpdateStackInput{
			StackName:    aws.String(template.StackName),
			TemplateURL:  aws.String(template.url()),
			Capabilities: capabilities,
		})
		if err != nil {
			fmt.Println("failed to update stack, " + err.Error())
		} else {
			fmt.Println("Response", resp)
			wait(svc, template.StackName, true)
		}
	} else {
		// Build the request with its input parameters
		resp, err := svc.CreateStack(ctx, &cloudformation.CreateStackInput{
			StackName:       aws.String(template.StackName),
			DisableRollback: aws.Bool(true),
			TemplateURL:     aws.String(template.url()),
			Capabilities:    capabilities,
		})

		if err != nil {
			fmt.Println("failed to update stack, " + err.Error())
		} else {
			fmt.Println("Response", resp)
			wait(svc, template.StackName, false)
		}
	}

}

func validate(template CFTemplate) error {
	cfn := cloudformation.NewFromConfig(cfg)
	_, err := cfn.ValidateTemplate(ctx, &cloudformation.ValidateTemplateInput{
		TemplateURL: aws.String(template.url()),
	})
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
	ctx = context.TODO()
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
