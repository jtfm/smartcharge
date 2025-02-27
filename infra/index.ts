import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";
import * as child_process from "child_process";
import { registerAutoTags } from "./autotag";

const config = new pulumi.Config();
registerAutoTags({
  "user:Project": pulumi.getProject(),
  "user:Stack": pulumi.getStack(),
  "pulumi:StackId": "Smartcharge"
});

// Create a DynamoDB table with a fixed partition key and sort key
const systemStatesTable = new aws.dynamodb.Table("system_states", {
  attributes: [
    {
      name: "partition_key", // Partition key name
      type: "N", // Number type, because we're using a fixed number "1"
    },
    {
      name: "start_time", // Sort key name
      type: "N", // Number type for UNIX timestamp
    },
  ],
  hashKey: "partition_key", // Set the fixed partition key to "partition_key"
  rangeKey: "start_time", // Set the sort key to "valid_from"
  billingMode: "PAY_PER_REQUEST",
});

const energyUsagesTable = new aws.dynamodb.Table("energy_usages", {
  attributes: [
    {
      name: "partition_key", // Partition key name
      type: "N", // Number type, because we're using a fixed number "1"
    },
    {
      name: "start_time", // Sort key name
      type: "N", // Number type for UNIX timestamp
    },
  ],
  hashKey: "partition_key", // Set the fixed partition key to "partition_key"
  rangeKey: "start_time", // Set the sort key to "valid_from"
  billingMode: "PAY_PER_REQUEST",
});

// Create IAM role with Lambda execution permission and attach the AWSLambdaBasicExecutionRole policy
const lambdaRole = new aws.iam.Role("smartcharge-lambda-role", {
  assumeRolePolicy: aws.iam.assumeRolePolicyForPrincipal({ Service: "lambda.amazonaws.com" }),
});

// Attach the AWS managed policy for basic Lambda execution
const basicExecutionPolicyAttachment = new aws.iam.RolePolicyAttachment("lambda-basic-execution-policy", {
  role: lambdaRole.name,
  policyArn: "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole",
});

// Define a policy allowing the Lambda function to write to the DynamoDB table
const lambdaPolicy = new aws.iam.Policy("dynamo-db-policy", {
  policy: pulumi.interpolate`{
    "Version": "2012-10-17",
    "Statement": [
      {
        "Sid": "DynamoDBIndexAndStreamAccess",
        "Effect": "Allow",
        "Action": [
          "dynamodb:GetShardIterator",
          "dynamodb:Scan",
          "dynamodb:Query",
          "dynamodb:DescribeStream",
          "dynamodb:GetRecords",
          "dynamodb:ListStreams"
        ],
        "Resource": [
          "${systemStatesTable.arn}",
          "${energyUsagesTable.arn}"
        ]
      },
      {
        "Sid": "DynamoDBWriteAccess",
        "Effect": "Allow",
        "Action": [
          "dynamodb:BatchGetItem",
          "dynamodb:BatchWriteItem",
          "dynamodb:ConditionCheckItem",
          "dynamodb:PutItem",
          "dynamodb:DescribeTable",
          "dynamodb:DeleteItem",
          "dynamodb:GetItem",
          "dynamodb:Scan",
          "dynamodb:Query",
          "dynamodb:UpdateItem"
        ],
        "Resource": [
          "${systemStatesTable.arn}",
          "${energyUsagesTable.arn}"
        ]
      },
      {
        "Sid": "DynamoDBDescribeLimitsAccess",
        "Effect": "Allow",
        "Action": "dynamodb:DescribeLimits",
        "Resource": [
          "${systemStatesTable.arn}",
          "${systemStatesTable.arn}/index/*",
          "${energyUsagesTable.arn}",
          "${energyUsagesTable.arn}/index/*"
        ]
      }
    ]
  }`
});

// Attach the policy to the Lambda role
new aws.iam.RolePolicyAttachment("dynamo-db-policy-attachment", {
  policyArn: lambdaPolicy.arn,
  role: lambdaRole.name,
});

// Build the Go binary
child_process.execSync(
  `cd ../smartcharge && ./build.sh && cd ../infra`,
  { stdio: "inherit" }
);

// Create a lambda function
const lambda = new aws.lambda.Function("smartcharge-lambda", {
  //name: lambdaFunctionName,
  description: "A lambda function optimises home battery charging based on solar generation and energy usage",
  code: new pulumi.asset.FileArchive(`../smartcharge/bin/app.zip`),
  role: lambdaRole.arn,
  handler: "main",
  runtime: aws.lambda.Runtime.CustomAL2023,
  memorySize: 128,
  timeout: 300,
  architectures: ["arm64"],
  loggingConfig: {
    logFormat: "JSON",
    applicationLogLevel: "INFO",
    systemLogLevel: "INFO",
  },
  environment: {
    variables: {
      CGO_ENABLED: "0",
      GOOS: "linux",
      GOARCH: "amd64",
      OCTOPUS_ACCOUNT_NUMBER: config.require("octopus_account_number"),
      OCTOPUS_USERNAME: config.require("octopus_username"),
      OCTOPUS_PASSWORD: config.require("octopus_password"),
      GIVENERGY_TOKEN: config.require("givenergy_token"),
      GIVENERGY_INVERTER_ID: config.require("givenergy_inverter_id"),
      SOLCAST_SITE_CODE: config.require("solcast_site_code"),
      SOLCAST_API_KEY: config.require("solcast_api_key"),
      DDB_SYSTEM_STATES_TABLE_NAME: systemStatesTable.name,
      DDB_ENERGY_USAGES_TABLE_NAME: energyUsagesTable.name,
    },
  },
});

//// Create a log group for the Lambda function with an expiration policy of 30 days
const logGroup = new aws.cloudwatch.LogGroup("smartcharge-lambda-log-group", {
  name: pulumi.interpolate`/aws/lambda/${lambda.name}`,
  retentionInDays: 30,
});

// Define an IAM role for the AWS Scheduler
const executionRole = new aws.iam.Role("scheduler-execution-role", {
  assumeRolePolicy: JSON.stringify({
    Version: "2012-10-17",
    Statement: [
      {
        Effect: "Allow",
        Principal: {
          Service: "scheduler.amazonaws.com",
        },
        Action: "sts:AssumeRole",
      },
    ],
  }),
});


const schedulerExecutionPolicyJson = lambda.arn.apply(arn =>
  JSON.stringify({
    Version: "2012-10-17",
    Statement: [
      {
        Effect: "Allow",
        Action: "lambda:InvokeFunction",
        Resource: arn,
      },
    ],
  })
);

new aws.iam.RolePolicy("scheduler-execution-policy", {
  role: executionRole.name,
  policy: schedulerExecutionPolicyJson,
})

new aws.scheduler.Schedule("30m-scheduler", {
  // Run the Lambda every half an hour
  scheduleExpression: "cron(0/30 * * * ? *)",
  flexibleTimeWindow: { mode: "OFF" }, // Ensures the schedule runs at the exact specified time
  target: {
    arn: lambda.arn, // ARN of the Lambda function
    roleArn: executionRole.arn, // ARN of the IAM role allowing the scheduler to invoke the Lambda
    input: JSON.stringify({ message: "Triggered by AWS Scheduler" }),
  },
});
