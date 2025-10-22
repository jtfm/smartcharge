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
  pointInTimeRecovery: {
    enabled: true,
  },
  deletionProtectionEnabled: true,
}, {
  protect: true,
  retainOnDelete: true,
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
  pointInTimeRecovery: {
    enabled: true,
  },
  deletionProtectionEnabled: true,
}, {
  protect: true,
  retainOnDelete: true,
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
  `cd ../smartcharge && ./build.sh && cd ../api && ./build.sh && cd ../infra`,
  { stdio: "inherit" }
);

// Create a lambda function
const lambda = new aws.lambda.Function("smartcharge-lambda", {
  //name: lambdaFunctionName,
  description: "A lambda function that optimises home battery charging based on solar generation and energy usage",
  code: new pulumi.asset.FileArchive(`../smartcharge/bin/app.zip`),
  role: lambdaRole.arn,
  handler: "bootstrap",
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
      GOARCH: "arm64",
      TZ: "Europe/London", // Set timezone to UK time (handles BST/GMT automatically)
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

// Create API Lambda function for frontend
const apiLambda = new aws.lambda.Function("smartcharge-api-lambda", {
  runtime: aws.lambda.Runtime.CustomAL2023,
  code: new pulumi.asset.AssetArchive({
    "bootstrap": new pulumi.asset.FileAsset("../api/bootstrap"),
  }),
  handler: "bootstrap",
  role: lambdaRole.arn,
  architectures: ["arm64"],
  environment: {
    variables: {
      DDB_ENERGY_USAGES_TABLE_NAME: energyUsagesTable.name,
      DDB_SYSTEM_STATES_TABLE_NAME: systemStatesTable.name,
    },
  },
  timeout: 30,
});

// Create Battery Dashboard Lambda function
const batteryDashboardLambda = new aws.lambda.Function("battery-dashboard-lambda", {
  runtime: aws.lambda.Runtime.CustomAL2023,
  code: new pulumi.asset.AssetArchive({
    "bootstrap": new pulumi.asset.FileAsset("../battery-dashboard/bootstrap"),
  }),
  handler: "bootstrap",
  role: lambdaRole.arn,
  architectures: ["x86_64"],
  environment: {
    variables: {
      DDB_SYSTEM_STATES_TABLE_NAME: systemStatesTable.name,
    },
  },
  timeout: 30,
  memorySize: 128,
});

// Create log group for battery dashboard lambda
const batteryDashboardLogGroup = new aws.cloudwatch.LogGroup("battery-dashboard-log-group", {
  name: pulumi.interpolate`/aws/lambda/${batteryDashboardLambda.name}`,
  retentionInDays: 30,
});

// Create Lambda Function URL for direct access to battery dashboard
const batteryDashboardFunctionUrl = new aws.lambda.FunctionUrl("battery-dashboard-url", {
  functionName: batteryDashboardLambda.name,
  authorizationType: "NONE",
  cors: {
    allowCredentials: false,
    allowHeaders: ["date", "keep-alive"],
    allowMethods: ["GET"],
    allowOrigins: ["*"],
    exposeHeaders: ["date", "keep-alive"],
    maxAge: 86400,
  },
});

// Create API Gateway
const api = new aws.apigateway.RestApi("smartcharge-api", {
  description: "Smartcharge Energy Dashboard API",
});

// Create API Gateway resource for energy endpoint
const energyResource = new aws.apigateway.Resource("energy-resource", {
  restApi: api.id,
  parentId: api.rootResourceId,
  pathPart: "energy",
});

const usageResource = new aws.apigateway.Resource("usage-resource", {
  restApi: api.id,
  parentId: energyResource.id,
  pathPart: "usage",
});

// Create method for GET /energy/usage
const usageMethod = new aws.apigateway.Method("usage-method", {
  restApi: api.id,
  resourceId: usageResource.id,
  httpMethod: "GET",
  authorization: "NONE",
});

// Create OPTIONS method for CORS
const usageOptionsMethod = new aws.apigateway.Method("usage-options-method", {
  restApi: api.id,
  resourceId: usageResource.id,
  httpMethod: "OPTIONS",
  authorization: "NONE",
});

// Create integration for GET method
const usageIntegration = new aws.apigateway.Integration("usage-integration", {
  restApi: api.id,
  resourceId: usageResource.id,
  httpMethod: usageMethod.httpMethod,
  integrationHttpMethod: "POST",
  type: "AWS_PROXY",
  uri: apiLambda.invokeArn,
});

// Create integration for OPTIONS method (CORS)
const usageOptionsIntegration = new aws.apigateway.Integration("usage-options-integration", {
  restApi: api.id,
  resourceId: usageResource.id,
  httpMethod: usageOptionsMethod.httpMethod,
  type: "MOCK",
  requestTemplates: {
    "application/json": '{"statusCode": 200}',
  },
});

// Create method response for OPTIONS
const usageOptionsMethodResponse = new aws.apigateway.MethodResponse("usage-options-method-response", {
  restApi: api.id,
  resourceId: usageResource.id,
  httpMethod: usageOptionsMethod.httpMethod,
  statusCode: "200",
  responseParameters: {
    "method.response.header.Access-Control-Allow-Headers": true,
    "method.response.header.Access-Control-Allow-Methods": true,
    "method.response.header.Access-Control-Allow-Origin": true,
  },
});

// Create integration response for OPTIONS
const usageOptionsIntegrationResponse = new aws.apigateway.IntegrationResponse("usage-options-integration-response", {
  restApi: api.id,
  resourceId: usageResource.id,
  httpMethod: usageOptionsMethod.httpMethod,
  statusCode: usageOptionsMethodResponse.statusCode,
  responseParameters: {
    "method.response.header.Access-Control-Allow-Headers": "'Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token'",
    "method.response.header.Access-Control-Allow-Methods": "'GET,OPTIONS'",
    "method.response.header.Access-Control-Allow-Origin": "'*'",
  },
});

// Grant API Gateway permission to invoke Lambda
const apiLambdaPermission = new aws.lambda.Permission("api-lambda-permission", {
  statementId: "AllowAPIGatewayInvoke",
  action: "lambda:InvokeFunction",
  function: apiLambda.name,
  principal: "apigateway.amazonaws.com",
  sourceArn: pulumi.interpolate`${api.executionArn}/*/*`,
});

// Create API Gateway resource for battery dashboard
const dashboardResource = new aws.apigateway.Resource("dashboard-resource", {
  restApi: api.id,
  parentId: api.rootResourceId,
  pathPart: "dashboard",
});

// Create method for GET /dashboard
const dashboardMethod = new aws.apigateway.Method("dashboard-method", {
  restApi: api.id,
  resourceId: dashboardResource.id,
  httpMethod: "GET",
  authorization: "NONE",
});

// Create OPTIONS method for CORS
const dashboardOptionsMethod = new aws.apigateway.Method("dashboard-options-method", {
  restApi: api.id,
  resourceId: dashboardResource.id,
  httpMethod: "OPTIONS",
  authorization: "NONE",
});

// Create integration for GET method
const dashboardIntegration = new aws.apigateway.Integration("dashboard-integration", {
  restApi: api.id,
  resourceId: dashboardResource.id,
  httpMethod: dashboardMethod.httpMethod,
  integrationHttpMethod: "POST",
  type: "AWS_PROXY",
  uri: batteryDashboardLambda.invokeArn,
});

// Create integration for OPTIONS method (CORS)
const dashboardOptionsIntegration = new aws.apigateway.Integration("dashboard-options-integration", {
  restApi: api.id,
  resourceId: dashboardResource.id,
  httpMethod: dashboardOptionsMethod.httpMethod,
  type: "MOCK",
  requestTemplates: {
    "application/json": '{"statusCode": 200}',
  },
});

// Create method response for OPTIONS
const dashboardOptionsMethodResponse = new aws.apigateway.MethodResponse("dashboard-options-method-response", {
  restApi: api.id,
  resourceId: dashboardResource.id,
  httpMethod: dashboardOptionsMethod.httpMethod,
  statusCode: "200",
  responseModels: {
    "application/json": "Empty",
  },
  responseParameters: {
    "method.response.header.Access-Control-Allow-Headers": true,
    "method.response.header.Access-Control-Allow-Methods": true,
    "method.response.header.Access-Control-Allow-Origin": true,
  },
});

// Create integration response for OPTIONS
const dashboardOptionsIntegrationResponse = new aws.apigateway.IntegrationResponse("dashboard-options-integration-response", {
  restApi: api.id,
  resourceId: dashboardResource.id,
  httpMethod: dashboardOptionsMethod.httpMethod,
  statusCode: dashboardOptionsMethodResponse.statusCode,
  responseParameters: {
    "method.response.header.Access-Control-Allow-Headers": "'Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token'",
    "method.response.header.Access-Control-Allow-Methods": "'GET,OPTIONS'",
    "method.response.header.Access-Control-Allow-Origin": "'*'",
  },
});

// Grant API Gateway permission to invoke Battery Dashboard Lambda
const batteryDashboardLambdaPermission = new aws.lambda.Permission("battery-dashboard-lambda-permission", {
  statementId: "AllowAPIGatewayInvokeDashboard",
  action: "lambda:InvokeFunction",
  function: batteryDashboardLambda.name,
  principal: "apigateway.amazonaws.com",
  sourceArn: pulumi.interpolate`${api.executionArn}/*/*`,
});

// Deploy API Gateway
const apiDeployment = new aws.apigateway.Deployment("api-deployment", {
  restApi: api.id,
}, {
  dependsOn: [
    usageMethod,
    usageOptionsMethod,
    usageIntegration,
    usageOptionsIntegration,
    dashboardMethod,
    dashboardOptionsMethod,
    dashboardIntegration,
    dashboardOptionsIntegration,
    dashboardOptionsMethodResponse,
    dashboardOptionsIntegrationResponse
  ],
});

// Create API Gateway Stage
const apiStage = new aws.apigateway.Stage("api-stage", {
  deployment: apiDeployment.id,
  restApi: api.id,
  stageName: "prod",
});

// Export API endpoint
export const apiEndpoint = pulumi.interpolate`https://${api.id}.execute-api.${aws.getRegionOutput().name}.amazonaws.com/prod`;
export const apiUrl = pulumi.interpolate`https://${api.id}.execute-api.${aws.getRegionOutput().name}.amazonaws.com/prod`;
export const batteryDashboardUrl = batteryDashboardFunctionUrl.functionUrl;
export const batteryDashboardApiUrl = pulumi.interpolate`https://${api.id}.execute-api.${aws.getRegionOutput().name}.amazonaws.com/prod/dashboard`;

// Note: Amplify frontend temporarily disabled. 
// To enable, add github_token to Pulumi config and uncomment Amplify resources above.
