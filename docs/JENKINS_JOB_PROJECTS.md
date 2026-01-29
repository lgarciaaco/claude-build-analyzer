# Jenkins Job Project Selection Guide

## Overview

Claude must select the appropriate Jenkins job project when analyzing Jenkins builds. This guide provides clear decision rules and usage instructions.

## Available Job Projects

### ocp4-konflux (Default)
- **Purpose**: Component image builds (ironic, oauth-server, etc.)
- **When to use**: Standard build failure analysis, component builds
- **User keywords**: "konflux", "build", "component", "image"
- **Example**: "analyze konflux build 48594"

### prepare-release-konflux
- **Purpose**: Release preparation builds (contains JIRA ticket references)
- **When to use**: Release preparation failures, JIRA ticket analysis
- **User keywords**: "prepare", "release", "prepare-release"
- **Example**: "tell me errors in prepare release 83975"

## Claude Decision Rules

### Step 1: Parse User Query
Look for these keywords in user input:

**prepare-release-konflux indicators:**
- "prepare"
- "release" 
- "prepare-release"
- "prepare release"

**ocp4-konflux indicators (default):**
- "konflux"
- "build"
- "component"
- No specific keywords found

### Step 2: Select Project
```
IF user mentions "prepare" OR "release" OR "prepare-release"
   THEN jobProject = "prepare-release-konflux"
ELSE
   jobProject = "ocp4-konflux" (default)
```

### Step 3: Add Parameter
Include `jobProject` parameter in ALL Jenkins MCP tool calls:

```json
{
  "buildNumber": 48594,
  "jobProject": "ocp4-konflux"
}
```

## Usage Examples

### Example 1: Component Build Analysis
**User**: "analyze konflux build 48594"
**Claude Logic**: No "prepare/release" keywords → Use default
**MCP Call**:
```json
{
  "buildNumber": 48594,
  "jobProject": "ocp4-konflux"
}
```

### Example 2: Release Build Analysis  
**User**: "tell me errors in prepare release 83975"
**Claude Logic**: Contains "prepare" and "release" keywords
**MCP Call**:
```json
{
  "buildNumber": 83975,
  "jobProject": "prepare-release-konflux"
}
```

### Example 3: Generic Build Query
**User**: "what failed in jenkins job 12345"
**Claude Logic**: No specific keywords → Use default
**MCP Call**:
```json
{
  "buildNumber": 12345,
  "jobProject": "ocp4-konflux"
}
```

## MCP Tool Parameter

Add this parameter to ALL Jenkins MCP tool calls:

```json
"jobProject": {
  "type": "string", 
  "description": "Jenkins job project (ocp4-konflux, prepare-release-konflux). Claude selects based on user context.",
  "default": "ocp4-konflux"
}
```

## Implementation Checklist

When calling Jenkins MCP tools, Claude must:

1. ✅ **Parse user query** for job project keywords
2. ✅ **Select job project** using decision rules above  
3. ✅ **Include jobProject parameter** in MCP call
4. ✅ **Document selection** in analysis response

**Never**: Call Jenkins MCP tools without the `jobProject` parameter after this implementation.