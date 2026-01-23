# CLAUDE.md - Quad-Server MCP Build Analysis System

## Essential Documentation

**📋 READ FIRST**: Every session should start by reading [docs/ANALYSIS_METHODOLOGY.md](docs/ANALYSIS_METHODOLOGY.md) which provides:
- Evidence-based investigation workflow
- Mandatory metadata-first protocol  
- Tool selection and usage patterns
- Performance optimization guidelines

## Project Purpose
This quad-server Go-based MCP system enables Claude to analyze OpenShift Container Platform build failures with natural language queries. The system uses four specialized servers for optimal performance and maintainability.

## Quick Start: Essential Query Order

**🔑 GOLDEN RULE: Always start with Metadata, optionally add Jenkins context, then go to Database**

### MANDATORY WORKFLOW - NO EXCEPTIONS

**🛑 BEFORE ANY BUILD ANALYSIS - EXECUTE THIS CHECKLIST:**

1. **STEP 1 (REQUIRED)**: Component validation using `mcp__ocp-metadata__search_components`
2. **STEP 2 (REQUIRED)**: Configuration retrieval using `mcp__ocp-metadata__get_component_metadata`  
3. **STEP 3 (CONDITIONAL)**: Jenkins context using `mcp__jenkins-server__*` tools - USE when:
   - User mentions specific Jenkins job numbers
   - Need build execution context beyond failure data
   - Want to extract build parameters (version, assembly)
4. **STEP 4 (CONDITIONAL)**: JIRA analysis using `mcp__jira-server__*` tools - USE when:
   - Jenkins logs reference OCPBUGS tickets or CVEs
   - Need security impact analysis for build failures
   - Want to correlate build issues with known bugs
5. **STEP 5 (ONLY AFTER 1&2)**: Build analysis using `mcp__build-analyzer__*` tools

**🚨 VIOLATION PREVENTION**: If you call ANY `mcp__build-analyzer__*` tool WITHOUT first completing steps 1 & 2, you are violating the protocol.

**Why this order is mandatory:**
- ⚡ **Performance**: Metadata queries are 2-5x faster than BigQuery operations
- 💰 **Cost Control**: Prevent expensive BigQuery operations on invalid components
- 🎯 **Accuracy**: Validate component names before database queries
- 📋 **Context**: Configuration understanding informs failure analysis

## Quad-Server Architecture

### Server Separation Strategy
```
┌─────────────────────────┐ ┌─────────────────────────┐ ┌─────────────────────────┐ ┌─────────────────────────┐
│    Build Analyzer       │ │    OCP Metadata         │ │    Jenkins Server       │ │    JIRA Server          │
│       Server            │ │       Server            │ │                         │ │                         │
├─────────────────────────┤ ├─────────────────────────┤ ├─────────────────────────┤ ├─────────────────────────┤
│ • BigQuery Integration  │ │ • Git Repository Mgmt   │ │ • Jenkins API Access    │ │ • JIRA API Access       │
│ • Build failure queries │ │ • YAML Metadata Parse   │ │ • Konflux build logs    │ │ • OCPBUGS ticket lookup │
│ • Log analysis & URLs   │ │ • Component configs     │ │ • Console log analysis  │ │ • CVE impact analysis   │
│ • Architecture patterns │ │ • Fuzzy search matching │ │ • Build correlation     │ │ • Security assessment   │
│ • Performance-focused   │ │ • Multi-version support │ │ • Pattern detection     │ │ • Issue correlation     │
└─────────────────────────┘ └─────────────────────────┘ └─────────────────────────┘ └─────────────────────────┘
```

### Key Architectural Benefits
- **Separation of Concerns**: Each server optimized for its specific data source
- **Independent Scaling**: Servers can scale and fail independently  
- **Technology Optimization**: BigQuery vs Git vs Jenkins vs JIRA API optimizations in each server
- **Maintainable Code**: Shared abstractions with focused implementations
- **Layered Analysis**: Jenkins execution + JIRA issue tracking + BigQuery build outcomes + Configuration metadata

## Data Sources and Responsibilities

### Build Analyzer Server
**Data Sources**: 
- BigQuery `openshift-art.events.builds` table (build metadata)
- BigQuery `openshift-art.events.taskruns` table (container logs)

**Responsibilities**:
- Query build failure data with concurrent operations
- Analyze actual container logs from TaskRun records
- Extract and parse build pipeline URLs
- Compare builds across time periods and architectures  
- Identify failure patterns and trends from log content

**Assembly Field Handling**:
- **Primary Table**: `builds` table contains `assembly` field (stream, test, standard, custom, preview)
- **Default Value**: Always defaults to "stream" for continuous builds when not specified
- **Assembly Types**: stream (default/continuous), test (assembly tests), standard (release), custom (no constraints), preview (pre-release)

### OCP Metadata Server  
**Data Source**: Git repositories of ocp-build-data (OpenShift 4.12-4.21)
**Responsibilities**:
- Parse component YAML configurations with flexible parsing
- Provide fuzzy search for component name resolution
- Track configuration changes across OpenShift versions
- Handle complex ocp-build-data YAML structures

### Jenkins Server
**Data Source**: Jenkins REST API at art-jenkins.apps.prod-stable-spoke1-dc-iad2.itup.redhat.com

**Authentication**: ✅ **CONFIGURED** - Jenkins credentials are available in `.mcp.json`
- **Username**: `lgarciaa` 
- **Token**: Pre-configured API token for full Jenkins access
- **Access Level**: Can retrieve logs, job details, and build parameters from all Konflux jobs

**Responsibilities**:
- Query Konflux build jobs under job/aos-cd-builds/job/build%252Focp4-konflux/
- Retrieve console logs with failure pattern analysis
- Extract build parameters (BUILD_VERSION, ASSEMBLY, DOOZER_DATA_GITREF)
- Correlate Jenkins execution with BigQuery build records
- Provide Jenkins-side context for build failures
- Support assembly-based filtering (stream, test, standard, custom, preview)

**Key Process Role**: Provides build execution context that enhances BigQuery failure analysis. Essential for extracting accurate version/assembly parameters from actual build jobs.

### JIRA Server
**Data Source**: JIRA REST API at issues.redhat.com

**Authentication**: ✅ **CONFIGURED** - JIRA credentials are available in `.mcp.json`
- **Token**: Pre-configured API Bearer token for Red Hat JIRA access
- **Access Level**: Can retrieve OCPBUGS tickets, CVE mappings, and security impact analysis

**Responsibilities**:
- Query OCPBUGS tickets referenced in Jenkins logs
- Analyze CVE impact and security assessments
- Extract component mappings from security issues
- Correlate build failures with known bugs and CVEs
- Provide security impact analysis for architecture planning
- Support issue lifecycle tracking and resolution status

**Key Process Role**: Links build failures to known issues and security vulnerabilities. Critical for understanding whether build failures are related to tracked bugs, CVEs, or require new issue creation.

## Assembly Parameter Usage

### Critical Assembly Usage Rules
1. **Extract from user prompt**: Look for explicit assembly mentions ("assembly test", "test assembly")
2. **Default to "stream"** when assembly not mentioned (continuous builds)
3. **Pass to build-analyzer tools**: Always include assembly parameter when calling build-analyzer tools
4. **Document in analysis**: Show which build type was analyzed

### Assembly Interpretation Examples
- "why did X fail in 4.21?" → `assembly="stream"` (default)
- "why did X fail in 21 assembly test?" → `assembly="test"`
- "X standard release failure" → `assembly="standard"`

## Jenkins Parameter Extraction

### CRITICAL VERSION DETECTION RULES
**🚨 NEVER assume default versions - ALWAYS extract from Jenkins job parameters**

When analyzing Jenkins jobs, you MUST:

1. **Extract BUILD_VERSION**: Use the `BUILD_VERSION` parameter from Jenkins job parameters
2. **Extract ASSEMBLY**: Use the `ASSEMBLY` parameter from Jenkins job parameters  
3. **Extract DOOZER_DATA_GITREF**: Contains version-specific branch information
4. **Apply extracted values**: Use these extracted values for all subsequent MCP calls

### Jenkins Parameter Extraction Examples
```json
{
  "BUILD_VERSION": "4.13",           // USE THIS for group parameter
  "ASSEMBLY": "test",                // USE THIS for assembly parameter
  "DOOZER_DATA_GITREF": "hermetic-migration-openshift-4.13-b91253d4"
}
```

### Version Parameter Usage
- Jenkins job 24021 shows `BUILD_VERSION: "4.13"` → Use `group="openshift-4.13"`
- Jenkins job 24021 shows `ASSEMBLY: "test"` → Use `assembly="test"`  
- **NEVER default to 4.21** when Jenkins parameters show different version

### Parameter Extraction Workflow
1. **Step 3A**: When using `mcp__jenkins-server__analyze_jenkins_logs`, extract job parameters
2. **Step 3B**: Parse `BUILD_VERSION` and `ASSEMBLY` from the Jenkins response
3. **Step 3C**: Update all subsequent parameters:
   - Jenkins `BUILD_VERSION: "4.13"` → Use `group="openshift-4.13"` in BigQuery calls
   - Jenkins `ASSEMBLY: "test"` → Use `assembly="test"` in BigQuery calls
4. **Step 4**: Use extracted values in all `mcp__build-analyzer__*` calls for accurate correlation

## MCP Query Order Protocol

### MANDATORY PROTOCOL: Metadata First, Database Second

**🚨 CRITICAL ENFORCEMENT: NO BUILD ANALYZER TOOLS WITHOUT METADATA VALIDATION**

Every build analysis request MUST follow this exact sequence:

#### STEP 1: Component Validation (MANDATORY)
**Tool**: `mcp__ocp-metadata__search_components`
**Purpose**: Validate and resolve exact component names
**Time**: 0.5-1 seconds
**Status**: ✅ REQUIRED before ANY other MCP calls

#### STEP 2: Configuration Context (MANDATORY)
**Tool**: `mcp__ocp-metadata__get_component_metadata` 
**Purpose**: Retrieve component configuration and build setup
**Time**: 0.5-2 seconds
**Status**: ✅ REQUIRED after Step 1, before Step 3

#### STEP 3: Jenkins Context (CONDITIONAL - AFTER STEPS 1 & 2)
**Tools**: `mcp__jenkins-server__*` (query_jenkins_builds, analyze_jenkins_logs, correlate_jenkins_builds)
**Purpose**: Extract build execution context and parameters from Jenkins jobs
**Time**: 1-3 seconds
**Status**: ✅ CONDITIONAL after Steps 1 & 2

**When to Use**:
- User mentions specific Jenkins job numbers (e.g., "job 24021")
- Need to extract BUILD_VERSION and ASSEMBLY from actual build execution
- Want Jenkins console log context beyond BigQuery failure data
- Correlating Jenkins job execution with BigQuery build outcomes

**Critical Process**: Always extract BUILD_VERSION and ASSEMBLY from Jenkins parameters to ensure version consistency in subsequent BigQuery queries

#### STEP 4: JIRA Analysis (CONDITIONAL - AFTER STEPS 1 & 2)
**Tools**: `mcp__jira-server__*` (get_issue, search_issues, get_issue_comments, analyze_security_impact)
**Purpose**: Extract JIRA issue context and CVE analysis when referenced in Jenkins logs
**Time**: 1-3 seconds
**Status**: ✅ CONDITIONAL after Steps 1 & 2

**When to Use**:
- Jenkins logs contain OCPBUGS ticket references (e.g., OCPBUGS-12345)
- CVE identifiers found in build failure context
- Security-related build failures requiring impact analysis
- Need to correlate build issues with existing bug reports

#### STEP 5: Build Analysis (ONLY AFTER STEPS 1 & 2)
**Tools**: `mcp__build-analyzer__*` (query_build_failures, analyze_build_logs, compare_builds)
**Purpose**: Expensive BigQuery operations for failure analysis
**Time**: 1-5 seconds + BigQuery costs
**Status**: ⛔ FORBIDDEN until Steps 1 & 2 completed

### PROTOCOL ENFORCEMENT RULES

**🛑 HARD STOP CONDITIONS:**
- If ANY `mcp__build-analyzer__*` tool called without prior `mcp__ocp-metadata__*` calls → **PROTOCOL VIOLATION**
- If component name validation skipped → **PROTOCOL VIOLATION**
- If configuration context retrieval skipped → **PROTOCOL VIOLATION**

**✅ ONLY EXCEPTIONS (No metadata validation required):**
- User provides specific build IDs for direct log analysis using `mcp__build-analyzer__analyze_build_logs`
- User provides specific build IDs for comparison using `mcp__build-analyzer__compare_builds`

### Protocol Benefits (Why This Is Mandatory)

1. **Cost Prevention**: Stop expensive BigQuery queries on invalid component names
2. **Performance Optimization**: Fast metadata validation before slow database operations
3. **Error Elimination**: Catch typos and invalid names immediately
4. **Context Enrichment**: Component configuration informs failure interpretation
5. **Resource Efficiency**: Minimize BigQuery usage and associated costs

## Tool Distribution

### Available MCP Tools

**🔒 VALIDATION REQUIRED: All build-analyzer tools require prior metadata validation**

#### OCP Metadata Server Tools (STEP 1 & 2)
**Must be called FIRST before any build analysis:**

- `mcp__ocp-metadata__search_components`: Component name validation and fuzzy search
- `mcp__ocp-metadata__get_component_metadata`: Component configuration retrieval

#### Jenkins Server Tools (STEP 3 - OPTIONAL CONTEXT)
**✅ OPTIONAL after Steps 1 & 2, provides Jenkins execution context:**

- `mcp__jenkins-server__query_jenkins_builds`: Query Konflux builds from Jenkins
- `mcp__jenkins-server__analyze_jenkins_logs`: Retrieve and analyze Jenkins console logs
- `mcp__jenkins-server__correlate_jenkins_builds`: Correlate Jenkins jobs with BigQuery data
- `mcp__jenkins-server__open_browser_links`: Open Konflux, Jenkins, and GitHub URLs in browser with safety validation

#### JIRA Server Tools (STEP 4 - OPTIONAL CONTEXT)
**✅ OPTIONAL after Steps 1 & 2, provides JIRA issue context:**

- `mcp__jira-server__get_issue`: Retrieve detailed OCPBUGS ticket information with security analysis
- `mcp__jira-server__search_issues`: Search JIRA issues by CVE, component, or JQL with pattern analysis  
- `mcp__jira-server__get_issue_comments`: Retrieve and analyze JIRA issue comments for technical insights
- `mcp__jira-server__analyze_security_impact`: Perform detailed security impact analysis for CVE-related issues

#### Build Analyzer Server Tools (STEP 5 - RESTRICTED ACCESS)
**⛔ FORBIDDEN without completing metadata validation first:**

- `mcp__build-analyzer__query_build_failures`: Build failure queries with filtering
- `mcp__build-analyzer__analyze_build_logs`: Container log analysis and URLs
- `mcp__build-analyzer__compare_builds`: Differential build analysis

**✅ EXCEPTIONS**: Direct build ID analysis (when user provides specific build IDs)

### CRITICAL VALIDATION REQUIREMENTS

**Before calling ANY `mcp__build-analyzer__*` tool:**
1. ✅ Component name validated via `mcp__ocp-metadata__search_components`
2. ✅ Component configuration retrieved via `mcp__ocp-metadata__get_component_metadata`
3. ✅ Assembly parameter correctly set based on user query
4. ✅ Version parameter matches user's OpenShift version request

**PROTOCOL VIOLATION CHECK**: If you attempt to call build-analyzer tools without metadata validation, you are violating cost-control and performance protocols.

## Evidence-Based Analysis Requirements

### CRITICAL ANALYSIS RULES
1. **Metadata validation FIRST**: Before ANY analysis, validate component via `mcp__ocp-metadata__search_components`
2. **Configuration context REQUIRED**: Retrieve setup via `mcp__ocp-metadata__get_component_metadata`
3. **Jenkins parameter extraction**: When Jenkins job mentioned, extract version/assembly parameters
4. **Build logs are mandatory**: Analysis conclusions ONLY after successful `mcp__build-analyzer__analyze_build_logs`
5. **No speculation without data**: MCP server errors during log retrieval = analysis failure
6. **Evidence-first approach**: Actual data over pattern assumptions
7. **Parameter consistency**: Use Jenkins-extracted parameters in BigQuery queries for accurate correlation

### Mandatory Protocol Validation
**🛑 BEFORE ANY BUILD ANALYSIS:**
1. ✅ Component name validation completed
2. ✅ Component configuration retrieved  
3. ✅ Only then proceed to build log analysis

### Failure Protocol
- If metadata validation skipped → **Report protocol violation and restart with metadata**
- If `mcp__build-analyzer__analyze_build_logs` returns error → **Report "Analysis failed - unable to retrieve build logs"**
- Do NOT speculate without actual build log data
- Do NOT conclude based only on metadata differences without build evidence

## Performance Characteristics

### Build Analyzer Server
- **Memory Usage**: 50-200MB (optimized for query operations)
- **Query Response**: 1-5 seconds with partition optimization
- **Concurrent Capacity**: 10+ simultaneous BigQuery operations

### OCP Metadata Server
- **Memory Usage**: 100-300MB (git repositories + cached metadata)
- **Startup Time**: 10-30 seconds (git repository initialization)
- **Query Response**: 0.5-2 seconds (cached metadata lookups)

### Jenkins Server
- **Memory Usage**: 20-50MB (HTTP client and caching)
- **Startup Time**: 1-2 seconds (HTTP client initialization)
- **Query Response**: 1-3 seconds (Jenkins API calls)

### JIRA Server
- **Memory Usage**: 15-40MB (HTTP client and caching)
- **Startup Time**: 1-2 seconds (JIRA API client initialization)
- **Query Response**: 1-3 seconds (JIRA API calls)

### Combined System
- **Total Memory**: 185-590MB (all four servers running)
- **Concurrent Load**: 50+ analysis requests per hour
- **Failure Resilience**: Independent server failures don't affect the others

## Integration with Art-Tools Ecosystem

### Data Flow Architecture
1. **Component Definition** (ocp-build-data) → Build Configuration
2. **Build Execution** (Konflux/Brew) → Build Records + Jenkins Jobs
3. **Data Aggregation** (BigQuery) → Searchable Build Database
4. **Jenkins Storage** (art-jenkins) → Console Logs + Job Metadata + Build Parameters
5. **Issue Tracking** (JIRA) → OCPBUGS tickets + CVE mappings + Security analysis
6. **Metadata Analysis** (OCP Metadata Server) → Configuration Intelligence
7. **Jenkins Analysis** (Jenkins Server) → Execution Context + Parameter Extraction + Log Analysis
8. **JIRA Analysis** (JIRA Server) → Issue Intelligence + CVE correlation + Security impact
9. **Build Analysis** (Build Analyzer Server) → Failure Intelligence with multi-source correlation
10. **User Queries** (Natural Language) → Evidence-Based Recommendations with comprehensive correlation

## Development Guidelines

### Adding New Build Analysis Features
- **Target Server**: Build Analyzer Server (`internal/mcp/server.go`)
- **Requirements**: BigQuery integration, concurrent processing, partition filtering

### Adding New Metadata Features  
- **Target Server**: OCP Metadata Server (`internal/metadata-server/server.go`)
- **Requirements**: Git repository access, YAML parsing, version management

### Adding New Jenkins Features
- **Target Server**: Jenkins Server (`internal/jenkins-server/server.go`)
- **Requirements**: Jenkins API integration, log pattern analysis, build correlation

### Adding New JIRA Features
- **Target Server**: JIRA Server (`internal/jira-server/server.go`)
- **Requirements**: JIRA REST API integration, issue analysis, security impact assessment

### Shared Functionality Development
- **Target Location**: `pkg/shared/` package
- **Requirements**: Maintain backwards compatibility, comprehensive error handling

## Analysis Methodology

**See [docs/ANALYSIS_METHODOLOGY.md](docs/ANALYSIS_METHODOLOGY.md)** for complete workflows including:
- Evidence-based investigation approach
- Component identification strategies  
- Generic failure analysis patterns
- Error handling protocols
- Response structure guidelines

## MCP Server Communication

### Server Deployment and Usage

**Communication Protocol**: MCP servers communicate via stdio (standard input/output)

**Starting the Servers**:
```bash
# Build Analyzer Server
./build-analyzer

# OCP Metadata Server  
./ocp-metadata-server

# Jenkins Server
./jenkins-server

# JIRA Server
./jira-server
```

**Integration with Claude Code**:
- Servers run as background processes communicating through stdio
- Claude Code sends JSON-RPC messages to server stdin
- Servers respond with JSON-RPC responses via stdout
- Each server maintains its own state and database connections

**Configuration Requirements**:
- **BigQuery Authentication**: Server requires Google Cloud credentials for BigQuery access
- **Git Repository Access**: OCP Metadata server needs access to ocp-build-data repositories
- **Jenkins Authentication**: ✅ **PRE-CONFIGURED** in `.mcp.json` with full API access
  - `JENKINS_USERNAME`: `lgarciaa`
  - `JENKINS_TOKEN`: Pre-configured API token
- **JIRA Authentication**: ✅ **PRE-CONFIGURED** in `.mcp.json` with full API access
  - `JIRA_TOKEN`: Pre-configured Bearer token for Red Hat JIRA access
- **Memory Allocation**: Ensure sufficient memory for concurrent operations (185-590MB total)

### Container Log Analysis Enhancement

**TaskRun Record Structure**:
```json
{
  "creation_time": "2025-11-06T13:27:09Z",
  "task": "build-images", 
  "task_run": "ose-4-21-component-h5pj5-build-images-1",
  "pipeline_run": "ose-4-21-dpu-intel-netsec-vsp-mq77f",
  "build_id": "ose-4-21-dpu-intel-netsec-vsp-mq77f-79012262-d513-4d03-8452-268bb5fec5f6",
  "containers": [
    {
      "name": "build",
      "exit_code": 1,
      "state": "terminated", 
      "reason": "Error",
      "log_output": "actual container log content here..."
    }
  ]
}
```

**Log Analysis Capabilities**:
- Automatic extraction of failed container logs
- Pattern-based error detection (permission, memory, network, timeout issues)
- Container-specific failure analysis
- Exit code and termination reason tracking

## Browser Link Opening Integration

### Overview
The Jenkins server now includes cross-platform browser link opening functionality through the `mcp__jenkins-server__open_browser_links` tool. This enables direct browser access to Konflux builds, Jenkins jobs, and GitHub repositories from Claude analysis results.

### Safety Features
- **Domain Validation**: Only allows trusted domains (Konflux, Jenkins, GitHub)
- **Protocol Filtering**: Restricts to HTTP/HTTPS protocols only
- **Rate Limiting**: Maximum 10 links per operation (configurable up to 20)
- **URL Sanitization**: Validates URL format and structure before opening

### Supported Domains
- `konflux-ui.apps.kflux-ocp-p01.7ayg.p1.openshiftapps.com` - Konflux UI links
- `art-jenkins.apps.prod-stable-spoke1-dc-iad2.itup.redhat.com` - Jenkins console links
- `github.com` - GitHub repositories and files
- `api.github.com` - GitHub API endpoints

### Cross-Platform Support
- **Windows**: Uses `rundll32` with URL protocol handler
- **macOS**: Uses `open` command
- **Linux**: Uses `xdg-open` command

### Usage Examples
```json
{
  "name": "open_browser_links",
  "arguments": {
    "urls": [
      "https://konflux-ui.apps.kflux-ocp-p01.7ayg.p1.openshiftapps.com/ns/ocp-art-tenant/applications/openshift-4-21/pipelineruns/ose-4-21-ironic-abc123",
      "https://art-jenkins.apps.prod-stable-spoke1-dc-iad2.itup.redhat.com/job/aos-cd-builds/job/build%252Focp4-konflux/24571/console"
    ],
    "maxLinks": 5
  }
}
```

### Response Format
```json
{
  "results": [
    {
      "url": "https://konflux-ui.apps...",
      "success": true
    },
    {
      "url": "http://malicious.com",
      "success": false,
      "error": "validation failed: domain malicious.com is not in allowed list"
    }
  ],
  "totalOpened": 1,
  "totalFailed": 1
}
```

## Quality Standards

### Analysis Requirements
- **Component Identification**: 95%+ accuracy in fuzzy matching
- **Evidence-Based Conclusions**: All conclusions supported by successful tool calls with actual log content
- **Response Completeness**: Address all requested components systematically
- **Error Resilience**: Graceful degradation with partial data availability
- **Container Log Analysis**: Extract and analyze actual container failure logs when available