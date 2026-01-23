# Build Failure Analysis Methodology

This guide provides a systematic, evidence-based approach for analyzing OpenShift Container Platform build failures using the tri-server MCP system.

## Core Principles

### 1. Evidence-First Analysis
- **NEVER speculate without data**: All conclusions must be supported by successful MCP tool calls
- **Build logs are required**: Analysis fails if `analyze_build_logs()` returns an error
- **Data over assumptions**: Let the evidence guide the investigation, not preconceived patterns

### 2. Systematic Investigation
- **Component identification** → **Failure patterns** → **Log analysis** → **Root cause correlation**
- **Multi-source validation**: Cross-reference BigQuery data with metadata configuration
- **Temporal analysis**: Consider timing, trends, and environmental changes

## Standard Analysis Workflow

### CRITICAL: Metadata-First Protocol

**🔑 ALWAYS query OCP Metadata Server before Build Analyzer Server**

This mandatory order ensures:
- **Performance**: 2-5x faster metadata validation (0.5-2s) before expensive BigQuery (1-5s)
- **Cost Control**: Avoid failed BigQuery operations on invalid component names  
- **Logic Flow**: Understand component configuration before analyzing build failures

**Optional Jenkins Context**: Use Jenkins Server tools when specific job numbers are mentioned or build execution context is needed.

### Step 1: Component Identification (OCP Metadata Server)

**Start here for EVERY analysis** - this step is mandatory:

**When to use fuzzy search**:
- User provides partial names ("etcd" instead of "etcd-operator")
- Functional terms ("monitoring", "oauth", "networking")
- Unclear or abbreviated component references

```bash
# ALWAYS START HERE: Fast component validation
mcp__ocp-metadata__search_components(query="user_input", version="4.21", maxResults=5)
```

**Then get exact metadata**:
```bash
# Required for configuration context
mcp__ocp-metadata__get_component_metadata(componentName="validated_name", version="4.21")
```

**Component name patterns**:
- Operators: `*-operator` (etcd-operator, cluster-dns-operator)
- Controllers: `*-controller`, `*-controller-manager`
- Core services: `oauth-server`, `openshift-apiserver`
- Infrastructure: `cluster-*`, `machine-*`

### Step 2: Jenkins Context (Optional - Jenkins Server)

**Use when**:
- User mentions specific Jenkins job numbers
- Need build execution parameters (BUILD_VERSION, ASSEMBLY)
- Want console log context beyond BigQuery data

```bash
# Extract build parameters from Jenkins jobs
mcp__jenkins-server__analyze_jenkins_logs(buildNumber=24021)
# Extract BUILD_VERSION and ASSEMBLY for subsequent BigQuery calls
```

### Step 3: Build Failure Pattern Analysis (Build Analyzer Server)

**ONLY after Step 1 metadata validation** - now query the expensive BigQuery database:

```bash
# Now safe to query BigQuery with validated component names
mcp__build-analyzer__query_build_failures(
  componentNames=["validated_component1", "validated_component2"],
  group="openshift-4.21",
  assembly="stream",  # or "test", "standard" based on user context
  days=7
)
```

**Performance Note**: This step takes 1-5 seconds + BigQuery compute costs

**Key data points to extract**:
- **Failure frequency**: Recent vs historical patterns
- **Architecture distribution**: Which platforms are affected
- **Timing correlation**: When did failures start/cluster
- **Assembly context**: Stream vs test vs standard builds

### Step 4: Configuration Analysis 

**Already completed in Step 1** - metadata was retrieved during component validation.

**Focus on analyzing the metadata**:
- **Recent changes**: Configuration modifications, dependency updates
- **Build environment**: Architecture settings, source repositories  
- **Dependencies**: Parent images, package requirements

### Query Decision Tree

```
User Query Analysis
    ├── Component name unclear? 
    │   ├── YES → search_components() [0.5-2s]
    │   └── NO → Skip to exact metadata
    │
    ├── Need component config?
    │   └── YES → get_component_metadata() [0.5-2s] 
    │
    ├── Need build failure data?
    │   └── YES → query_build_failures() [1-5s + BigQuery cost]
    │
    └── Need specific error logs?
        └── YES → analyze_build_logs() [2-10s + BigQuery cost]
```

### Performance Expectations

| Operation | Server | Time | Cost |
|-----------|--------|------|------|
| search_components | Metadata | 0.5-2s | Local Git (free) |
| get_component_metadata | Metadata | 0.5-2s | Local Git (free) |
| analyze_jenkins_logs | Jenkins | 1-3s | Jenkins API (free) |
| query_build_failures | Build Analyzer | 1-5s | BigQuery compute |
| analyze_build_logs | Build Analyzer | 2-10s | BigQuery compute |

**Total optimal analysis time**: 5-20 seconds following metadata-first protocol

### Step 5: Build Log Investigation

**CRITICAL**: This step is mandatory for conclusions
```
build-analyzer → analyze_build_logs(componentName="component", group="4.21", assembly="stream")
```

**If this fails**: Report "Analysis failed - unable to retrieve build logs due to MCP server error"
**If successful**: Extract specific error patterns, pipeline URLs, failure context

### Step 6: Evidence Correlation

**Data correlation approach**:
- **Timing**: Do metadata changes align with failure start times?
- **Scope**: Are multiple components affected by similar issues?
- **Environment**: Are failures architecture or assembly-specific?

## Assembly Context Handling

### Assembly Parameter Interpretation

| User Input | Assembly Parameter | Context |
|------------|-------------------|---------|
| "why did X fail in 4.21?" | `assembly="stream"` | Default continuous builds |
| "X failing in 21 assembly test" | `assembly="test"` | Assembly test builds |
| "X standard release failure" | `assembly="standard"` | Official release builds |
| "custom assembly issue" | `assembly="custom"` | Custom builds |

### Assembly Usage Rules
1. **Extract from user prompt**: Look for explicit assembly mentions
2. **Default to "stream"**: When assembly not specified
3. **Pass to all tools**: Both query_build_failures and analyze_build_logs
4. **Document in response**: Clarify which assembly was analyzed

## Common Investigation Patterns

### Single Component Failure
1. **Check timing**: Recent failure or ongoing issue?
2. **Architecture analysis**: All platforms or specific ones?
3. **Configuration review**: Recent metadata changes?
4. **Log analysis**: What do the build logs reveal?

### Multiple Component Failure
1. **Common factors**: Shared parent images, dependencies, timing?
2. **Scope assessment**: Related components or random distribution?
3. **Infrastructure correlation**: Build system issues or component-specific?

### Architecture-Specific Issues
1. **Platform comparison**: Success rates across x86_64, aarch64, ppc64le, s390x
2. **Cross-compilation**: CGO or platform-specific dependencies?
3. **Base image availability**: Platform-specific parent image issues?

## Error Classification Framework

### Failure Severity Assessment
**Critical** (immediate attention):
- 100% failure rate on primary architectures
- Multiple core components affected
- Build system infrastructure issues

**Standard** (scheduled resolution):
- Partial failure rates with some successes
- Single component issues
- Non-critical platform failures

**Low** (monitoring):
- Intermittent failures with low frequency
- Secondary architecture issues
- Test-only failures

### Temporal Classification
**Recent** (< 3 days):
- Likely configuration or infrastructure changes
- Check for recent metadata modifications
- Look for parent image or dependency updates

**Ongoing** (> 7 days):
- Systematic issues requiring deeper investigation
- Check for resource constraints or architectural problems
- Consider long-term trend analysis

## Tool Selection Guide

### Build Analyzer Server Tools

**query_build_failures**:
- **Use for**: Pattern analysis, failure frequency, architecture breakdown
- **Best for**: Understanding scope and timing of issues
- **Performance**: 1-5 seconds for multi-component analysis

**analyze_build_logs**:
- **Use for**: Specific error investigation, log URLs, failure details
- **Best for**: Root cause identification and evidence gathering
- **Required**: Must succeed for valid analysis conclusions

**compare_builds**:
- **Use for**: Before/after analysis, configuration differences
- **Best for**: Understanding what changed between working and failing builds

### OCP Metadata Server Tools

**search_components**:
- **Use for**: Component name resolution, fuzzy matching
- **Best for**: Handling unclear or partial component names

**get_component_metadata**:
- **Use for**: Configuration analysis, dependency review
- **Best for**: Understanding component setup and recent changes

### Jenkins Server Tools

**query_jenkins_builds**:
- **Use for**: Finding Konflux builds in Jenkins, filtering by status/time
- **Best for**: Locating specific build jobs and parameters

**analyze_jenkins_logs**:
- **Use for**: Extracting build parameters, console log analysis
- **Best for**: Getting BUILD_VERSION/ASSEMBLY for accurate BigQuery correlation

**correlate_jenkins_builds**:
- **Use for**: Linking Jenkins execution with BigQuery build records
- **Best for**: Cross-referencing Jenkins and BigQuery data

## Response Structure Template

```
Build Failure Analysis for [N] Components (Assembly: [stream/test/standard]):

## Component: [name]
- **Failure Pattern**: [Data-driven description from build failures]
- **Evidence**: [Specific findings from build logs and metadata]
- **Root Cause**: [Conclusion based on evidence correlation]
- **Recommendation**: [Actionable next steps]

## Component: [next]
[Same structure]

## Summary
- **Common Factors**: [If multiple components share issues]
- **Priority Actions**: [Most critical fixes needed]
- **Investigation Notes**: [Areas requiring further analysis]
```

## Error Handling Protocol

### MCP Tool Failures
- **If search_components fails**: "Unable to identify component - please provide exact component name"
- **If query_build_failures fails**: "Unable to retrieve build failure data due to server error"
- **If analyze_build_logs fails**: "Analysis failed - unable to retrieve build logs due to MCP server error"
- **If get_component_metadata fails**: "Unable to retrieve component configuration"

### Partial Data Scenarios
- **Some tools succeed**: Provide analysis based on available data, note limitations
- **Inconclusive evidence**: Report findings without speculation, suggest further investigation
- **Conflicting data**: Present all findings, note discrepancies, recommend verification

## Quality Standards

### Analysis Requirements
- **Evidence-based conclusions**: Every claim supported by tool data
- **Complete component coverage**: Address all requested components
- **Clear uncertainty handling**: Distinguish between known and unknown factors
- **Actionable recommendations**: Specific next steps for resolution

### Validation Checklist
- [ ] All component names validated via search or metadata lookup
- [ ] Assembly parameter correctly interpreted and applied
- [ ] Build logs successfully retrieved and analyzed
- [ ] Timing correlation performed between failures and changes
- [ ] Architecture-specific patterns identified where relevant
- [ ] Recommendations are specific and actionable