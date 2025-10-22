# Lambda Runtime Migration Summary

## Overview
All AWS Lambda functions in the SmartCharge project have been migrated from the deprecated `go1.x` runtime to the modern `provided.al2023` (CustomAL2023) runtime.

## Changes Made

### 1. Infrastructure Updates (`infra/index.ts`)

#### Main SmartCharge Lambda
- **Runtime**: Updated to `aws.lambda.Runtime.CustomAL2023`
- **Handler**: Changed from `"main"` to `"bootstrap"`
- **Architecture**: ARM64 (already configured)
- **Environment**: Fixed `GOARCH` from `"amd64"` to `"arm64"` to match architecture

#### API Lambda
- **Runtime**: Updated from `aws.lambda.Runtime.Go1dx` to `aws.lambda.Runtime.CustomAL2023`
- **Handler**: Changed from `"main"` to `"bootstrap"`
- **Code**: Updated to use single `bootstrap` binary instead of file archive
- **Architecture**: Added ARM64 for better performance and cost savings

#### Battery Dashboard Lambda
- **Runtime**: Updated from `"provided.al2"` to `aws.lambda.Runtime.CustomAL2023`
- **Architecture**: Added ARM64 for consistency

### 2. Build Script Updates

#### API Lambda (`api/build.sh`)
- **Created**: New build script for the API lambda
- **Target**: ARM64 Linux (`GOOS=linux GOARCH=arm64`)
- **Output**: `bootstrap` binary (required for custom runtime)
- **Optimization**: Added build flags `-ldflags="-s -w"` for smaller binaries

#### Battery Dashboard Lambda (`battery-dashboard/build.sh`)
- **Updated**: Changed target from AMD64 to ARM64
- **Target**: ARM64 Linux (`GOOS=linux GOARCH=arm64`)
- **Maintained**: ZIP package creation for deployment

#### SmartCharge Lambda (`smartcharge/build.sh`)
- **No Changes**: Already correctly configured for ARM64 and bootstrap binary

### 3. Infrastructure Build Process

#### Updated Build Command
```typescript
child_process.execSync(
  `cd ../smartcharge && ./build.sh && cd ../api && ./build.sh && cd ../infra`,
  { stdio: "inherit" }
);
```
- **Added**: API lambda build step to the infrastructure deployment process

### 4. Module Configuration (`go.work`)
- **Added**: API module to the workspace
- **Added**: Battery dashboard module to the workspace
- **Structure**: Now includes all four Go modules (smartcharge, core, api, battery-dashboard)

### 5. Package Fixes

#### API Lambda (`api/main.go`)
- **Fixed**: Removed duplicate package declaration
- **Package**: Corrected from `package api` + `package main` to just `package main`

### 6. Build Orchestration

#### Global Build Script (`build-all.sh`)
- **Updated**: Now builds both API and Battery Dashboard lambdas
- **Process**: Uses individual build scripts for each component
- **Validation**: Added error checking for each build step

#### Deployment Script (`deploy-dashboard.sh`)
- **Updated**: Builds both API and Battery Dashboard before deployment
- **Order**: API → Battery Dashboard → Infrastructure deployment

## Benefits

### Performance
- **ARM64 Architecture**: Up to 20% better price/performance compared to x86_64
- **Smaller Binaries**: Optimized build flags reduce package size
- **Faster Cold Starts**: Custom runtime with optimized Go binaries

### Cost Savings
- **ARM64 Pricing**: Lower cost per GB-second of compute time
- **Efficiency**: Reduced memory usage with optimized binaries

### Future-Proofing
- **Modern Runtime**: Using latest AWS Lambda runtime capabilities
- **Long-term Support**: CustomAL2023 has longer support lifecycle than Go 1.x
- **Flexibility**: Custom runtime allows for greater control over the execution environment

### Consistency
- **Unified Architecture**: All lambdas now use ARM64
- **Standard Build Process**: Consistent build scripts across all components
- **Shared Dependencies**: Proper module workspace configuration

## Deployment Instructions

### Quick Deploy
```bash
# Deploy everything with runtime updates
./deploy-dashboard.sh
```

### Manual Build and Deploy
```bash
# Build all components
./build-all.sh

# Deploy infrastructure
cd infra
pulumi up
```

### Individual Component Builds
```bash
# Main smartcharge lambda
cd smartcharge && ./build.sh

# API lambda
cd api && ./build.sh

# Battery dashboard lambda
cd battery-dashboard && ./build.sh
```

## Verification

After deployment, all lambda functions should:
1. **Runtime**: Show `provided.al2023` in AWS Console
2. **Architecture**: Show `arm64` in AWS Console
3. **Handler**: Show `bootstrap` in configuration
4. **Function**: Execute successfully without errors

## Rollback Plan

If issues arise, the infrastructure can be reverted by:
1. Changing runtime back to previous values in `infra/index.ts`
2. Updating handlers as needed
3. Running `pulumi up` to deploy changes

However, note that `go1.x` runtime is deprecated and should not be used for new deployments.

## Dependencies

All lambda functions require:
- **Go 1.21+**: For building the applications
- **AWS Lambda Go SDK**: For runtime integration
- **ARM64 Build Environment**: For cross-compilation (handled automatically by build scripts)

## Testing

Each lambda function includes:
- **Build Verification**: Build scripts validate successful compilation
- **Unit Tests**: Individual test files for core functionality
- **Integration Testing**: Can be tested via AWS Console or CLI after deployment