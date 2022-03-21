package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/joho/godotenv"
)

var step = 0

func init() {

	err := godotenv.Load(".env")

	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

// --- Keys ---

type AWSAccessAndSecretKeys struct {
	AccessKey       *string
	SecretAccessKey *string
}

func CreateTrustPolicyDoc(resourceArn string) ([]byte, error) {
	policy := TrustPolicyDocument{
		Version: "2012-10-17",
		Statement: []TrustStatementEntry{
			{
				Effect: "Allow",
				Action: []string{
					"sts:AssumeRole",
				},
				Principal: TrustStatementPrincipal{
					AWS: resourceArn, //"arn:aws:iam::" + accountNumber + "user/" + targetIAMUserName,
				},
			},
		},
	}

	b, err := json.Marshal(&policy)

	return b, err
}

func CreateBootstrapperRole(resourceArn string) ([]byte, error) {
	policy := PolicyDocument{
		Version: "2012-10-17",
		Statement: []StatementEntry{
			{
				Effect: "Allow",
				Action: []string{
					"s3:CreateBucket",
					"s3:GetObject",
					"s3:GetObjectAcl",
					"s3:ListBucket",
					"ecr:*",
					//	"sts:AssumeRole",
				},
				Resource: resourceArn,
			},
		},
	}

	b, err := json.Marshal(&policy)

	return b, err
}

type TrustPolicyDocument struct {
	Version   string
	Statement []TrustStatementEntry
}

type TrustStatementEntry struct {
	Effect    string
	Action    []string
	Principal TrustStatementPrincipal
}

type TrustStatementPrincipal struct {
	AWS string
}

// PolicyDocument is our definition of our policies to be uploaded to IAM.
type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
}

// StatementEntry will dictate what this policy will allow or not allow.
type StatementEntry struct {
	Effect   string
	Action   []string
	Resource string
}

type ErrorLine struct {
	Error       string      `json:"error"`
	ErrorDetail ErrorDetail `json:"errorDetail"`
}

type ErrorDetail struct {
	Message string `json:"message"`
}

func incStep() int {
	step++
	return step
}

func main() {
	fmt.Printf("Step %v. Initialize ENV Variables\n", incStep())

	var trustPolicyDocumentName = os.Getenv("FIRST_POLICY_NAME")
	var bootstrapPolicyName = os.Getenv("AWS_BOOTSTRAP_ROLE_NAME")
	var bucketName = os.Getenv("S3_BUCKET_NAME")
	var region = os.Getenv("REGION")
	var dockerRegistryUserID = os.Getenv("DOCKER_REGISTRY_USER_ID")

	if !(trustPolicyDocumentName != "" && bootstrapPolicyName != "" && bucketName != "" && region != "") {
		log.Fatal("Please provide env values for trustPolicyDocumentName, bootstrapPolicyName, bucketName, and region.")
	}

	fmt.Printf("Step %v. Initialize default profile clients (IAM & STS) and getting user account\n", incStep())
	defaultCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-2"))
	if err != nil {
		log.Fatal(err)
	}
	defaultSTSClient := sts.NewFromConfig(defaultCfg)
	defaultAccount, err := defaultSTSClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Default account arn: %v\n", *defaultAccount.Arn)

	fmt.Printf("Step %v. Deleting potentially prexisting roles/policies\n", incStep())
	defaultIAMClient := iam.NewFromConfig(defaultCfg)
	defaultIAMClient.DetachRolePolicy(context.TODO(), &iam.DetachRolePolicyInput{
		RoleName:  aws.String(trustPolicyDocumentName),
		PolicyArn: aws.String("arn:aws:iam::" + *defaultAccount.Account + ":policy/" + bootstrapPolicyName),
	})
	_, err = defaultIAMClient.DeleteRole(context.TODO(), &iam.DeleteRoleInput{
		RoleName: aws.String(trustPolicyDocumentName),
	})
	_, err = defaultIAMClient.DeletePolicy(context.TODO(), &iam.DeletePolicyInput{
		PolicyArn: aws.String("arn:aws:iam::" + *defaultAccount.Account + ":policy/" + bootstrapPolicyName),
	})

	fmt.Printf("Step %v. Creating policy documents and policy\n", incStep())
	fmt.Println("If an error has occured on this step, it's likely the policy/policy doc has already been made.")

	// policy doc to attach to policy to assume
	rolePolicyDoc, err := CreateBootstrapperRole("*")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// doc to add to role to be assumed
	trustRelationshipDocument, err := CreateTrustPolicyDoc(*defaultAccount.Arn)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	trustPolicyInfo, err := defaultIAMClient.CreatePolicy(context.TODO(), &iam.CreatePolicyInput{
		PolicyDocument: aws.String(string(rolePolicyDoc)),
		PolicyName:     aws.String(bootstrapPolicyName), //trustPolicyDocumentName),
	})

	if err != nil {
		if strings.Contains(err.Error(), "EntityAlreadyExists") {
			fmt.Printf("Trust Policy: {%v} already exists...", trustPolicyInfo.Policy.PolicyName)
			return
		}
		fmt.Printf("defaultIAMClient.CreatePolicy() error:\n%v", err.Error())
		return
	}
	//fmt.Printf("New policy:\n [PolicyName]: %v\n", result.Policy.PolicyName)

	if err != nil {
		fmt.Print(err)
	}

	fmt.Printf("---\n")

	fmt.Printf("Step %v. Creating Bootstrapper Role:\n", incStep())
	bootstrapRole, err := defaultIAMClient.CreateRole(context.TODO(), &iam.CreateRoleInput{
		RoleName:                 aws.String(trustPolicyDocumentName),
		AssumeRolePolicyDocument: aws.String(string(trustRelationshipDocument)),
		Description:              aws.String("Temporary Role for IAM User Bootstrap to assume for creating S3 for Terraform"),
	})

	if err != nil {
		fmt.Printf("Could not create Bootstrapper role")
		fmt.Printf(err.Error())
		return
	}

	fmt.Printf("Step %v. Attaching role to policy:\n", incStep())

	policyArn := "arn:aws:iam::" + *defaultAccount.Account + ":policy/" + *trustPolicyInfo.Policy.PolicyName
	fmt.Printf("Policy arn: %v\n", policyArn)
	_, err = defaultIAMClient.AttachRolePolicy(context.TODO(), &iam.AttachRolePolicyInput{
		PolicyArn: aws.String(policyArn), //trustPolicyInfo.Policy.Arn,
		RoleName:  aws.String(trustPolicyDocumentName),
	})
	if err != nil {
		fmt.Printf("Could not attach role policy")
		fmt.Printf(err.Error())
		return
	}

	fmt.Printf("First Policy Attached!\n")

	fmt.Printf("Step %v. Assuming role:\n", incStep())
	fmt.Println("If an error occurs here, please try again")

	time.Sleep(5 * time.Second)

	_, err = defaultSTSClient.AssumeRole(context.TODO(), &sts.AssumeRoleInput{
		RoleArn:         bootstrapRole.Role.Arn,
		RoleSessionName: aws.String("BootstrapUserSession"),
	})

	if err != nil {
		fmt.Printf("Could not assume role")
		log.Fatal(err)
	}
	fmt.Printf("Step %v. Creating s3 bucket:\n", incStep())
	s3Config, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-2"))
	if err != nil {
		log.Fatal(err)
	}

	s3Client := s3.NewFromConfig(s3Config)

	s3input := &s3.CreateBucketInput{
		Bucket:                    &bucketName,
		CreateBucketConfiguration: &types.CreateBucketConfiguration{LocationConstraint: types.BucketLocationConstraint(region)},
	}
	_, err = s3Client.CreateBucket(context.TODO(), s3input)
	if err != nil {
		fmt.Printf("Could not create bucket: %v\n", bucketName)
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Successfully created bucket: %v\n", bucketName)
	}
	fmt.Printf("Step %v. Deleting created roles/policies\n", incStep())
	defaultIAMClient.DetachRolePolicy(context.TODO(), &iam.DetachRolePolicyInput{
		RoleName:  aws.String(trustPolicyDocumentName),
		PolicyArn: aws.String("arn:aws:iam::" + *defaultAccount.Account + ":policy/" + bootstrapPolicyName),
	})
	_, err = defaultIAMClient.DeleteRole(context.TODO(), &iam.DeleteRoleInput{
		RoleName: aws.String(trustPolicyDocumentName),
	})
	_, err = defaultIAMClient.DeletePolicy(context.TODO(), &iam.DeletePolicyInput{
		PolicyArn: aws.String("arn:aws:iam::" + *defaultAccount.Account + ":policy/" + bootstrapPolicyName),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Step %v. Creating .tar of IMG:\n", incStep())

	tar, err := archive.TarWithOptions("./lambda_app", &archive.TarOptions{})
	if err != nil {
		fmt.Println(err)
	}
	opts := types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       []string{dockerRegistryUserID + "/lambda_app"},
		Remove:     true,
	}
	res, err := client.ImageBuild(context.TODO(), tar, opts)
	if err != nil {
		fmt.Println(err)
	}
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		// lastLine = scanner.Text()
		fmt.Println(scanner.Text())
	}

	fmt.Printf("Step %v. Creating ECR:\n", incStep())
	ecrConfig, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-2"))
	if err != nil {
		log.Fatal(err)
	}
	ecrClient := ecr.NewFromConfig(ecrConfig)
	ecrClient.CreateRepository(context.TODO(), &ecr.CreateRepositoryInput{
		RepositoryName: aws.String("CYDERES"),
	})

	// t := time.Now()
	// tstring := t.String()

	res, err = ecrClient.PutImage(context.TODO(), &ecr.PutImageInput{
		RepositoryName: aws.String("CYDERES"),
		ImageTag:       aws.String(dockerRegistryUserID + "/lambda_app"),
		ImageManifest:  aws.String("{\n   \"schemaVersion\": 2,\n   \"mediaType\": \"application/vnd.docker.distribution.manifest.list.v2+json\",\n   \"manifests\": [\n      {\n         \"mediaType\": \"application/vnd.docker.distribution.manifest.v2+json\",\n         \"size\": 527,\n         \"digest\": \"sha256:dca71257cd2e72840a21f0323234bb2e33fea6d949fa0f21c5102146f583486b\",\n         \"platform\": {\n            \"architecture\": \"amd64\",\n            \"os\": \"linux\"\n         }\n      },\n      {\n         \"mediaType\": \"application/vnd.docker.distribution.manifest.v2+json\",\n         \"size\": 527,\n         \"digest\": \"sha256:9cd47e9327430990c932b19596f8760e7d1a0be0311bb31bab3170bec5f27358\",\n         \"platform\": {\n            \"architecture\": \"arm\",\n            \"os\": \"linux\",\n            \"variant\": \"v5\"\n         }\n      },\n      {\n         \"mediaType\": \"application/vnd.docker.distribution.manifest.v2+json\",\n         \"size\": 527,\n         \"digest\": \"sha256:842295d11871c16bbce4d30cabc9b0f1e0cc40e49975f538179529d7798f77d8\",\n         \"platform\": {\n            \"architecture\": \"arm\",\n            \"os\": \"linux\",\n            \"variant\": \"v6\"\n         }\n      },\n      {\n         \"mediaType\": \"application/vnd.docker.distribution.manifest.v2+json\",\n         \"size\": 527,\n         \"digest\": \"sha256:0dd359f0ea0f644cbc1aa467681654c6b4332015ae37af2916b0dfb73b83fd52\",\n         \"platform\": {\n            \"architecture\": \"arm\",\n            \"os\": \"linux\",\n            \"variant\": \"v7\"\n         }\n      },\n      {\n         \"mediaType\": \"application/vnd.docker.distribution.manifest.v2+json\",\n         \"size\": 527,\n         \"digest\": \"sha256:121373e88baca4c1ef533014de2759e002961de035607dd35d00886b052e37cf\",\n         \"platform\": {\n            \"architecture\": \"arm64\",\n            \"os\": \"linux\",\n            \"variant\": \"v8\"\n         }\n      },\n      {\n         \"mediaType\": \"application/vnd.docker.distribution.manifest.v2+json\",\n         \"size\": 527,\n         \"digest\": \"sha256:ccff0c7e8498c0bd8d4705e663084c25810fd064a184671a050e1a43b86fb091\",\n         \"platform\": {\n            \"architecture\": \"386\",\n            \"os\": \"linux\"\n         }\n      },\n      {\n         \"mediaType\": \"application/vnd.docker.distribution.manifest.v2+json\",\n         \"size\": 527,\n         \"digest\": \"sha256:0dc4e9a14237cae2d8e96e9e310116091c5ed4934448d7cfd22b122778964f11\",\n         \"platform\": {\n            \"architecture\": \"mips64le\",\n            \"os\": \"linux\"\n         }\n      },\n      {\n         \"mediaType\": \"application/vnd.docker.distribution.manifest.v2+json\",\n         \"size\": 528,\n         \"digest\": \"sha256:04ebe37e000dcd9b1386af0e2d9aad726cbd1581f82067bea5cd2532b1f06310\",\n         \"platform\": {\n            \"architecture\": \"ppc64le\",\n            \"os\": \"linux\"\n         }\n      },\n      {\n         \"mediaType\": \"application/vnd.docker.distribution.manifest.v2+json\",\n         \"size\": 528,\n         \"digest\": \"sha256:c10e75f6e5442f446b7c053ff2f360a4052f759c59be9a4c7d144f60207c6eda\",\n         \"platform\": {\n            \"architecture\": \"s390x\",\n            \"os\": \"linux\"\n         }\n      }\n   ]\n}\n"),
	})

	if err != nil {
		fmt.Printf(err.Error())
	}

	fmt.Println(res)

}
