# Claude Build Analyzer

A high-performance tri-server MCP system written in Go that enables Claude to analyze OpenShift Container Platform build failures with natural language queries.

## Overview

This system provides Claude with comprehensive build failure analysis through three specialized MCP servers:

- **Build Analyzer Server**: BigQuery-based build failure analysis and log correlation
- **OCP Metadata Server**: Component metadata access from ocp-build-data repositories
- **Jenkins Server**: Jenkins API access for build context and console log analysis

**Example Usage**: Ask Claude "why did ironic fail to build in 4.21?" and get detailed analysis correlating build failures with component configuration changes.

## Architecture

### Tri-Server Design

```
┌─────────────────────┐    ┌──────────────────────┐    ┌─────────────────────┐
│   Build Analyzer    │    │   OCP Metadata       │    │   Jenkins Server    │
│      Server         │    │      Server          │    │                     │
├─────────────────────┤    ├──────────────────────┤    ├─────────────────────┤
│ • BigQuery queries  │    │ • Component configs  │    │ • Jenkins API       │
│ • Build log analysis│    │ • Fuzzy search       │    │ • Console logs      │
│ • Failure patterns  │    │ • YAML parsing       │    │ • Build correlation │
│ • Architecture data │    │ • Multi-version Git  │    │ • Parameter extract │
└─────────────────────┘    └──────────────────────┘    └─────────────────────┘
           │                           │                           │
           └─────────────────── Claude ────────────────────────────┘
```

### Key Benefits
- **Separation of Concerns**: Each server optimized for its data source
- **Independent Scaling**: Servers can be scaled independently
- **Focused Performance**: BigQuery vs Git repository optimizations
- **Maintainable**: Clean interfaces with shared abstractions

## Prerequisites

1. **Go 1.21+**
2. **Google Cloud credentials** with BigQuery access to `openshift-art` project
3. **Git access** to ocp-build-data repository (read-only)
4. **Claude Code** with MCP support

## Installation

1. **Clone the repository**:
```bash
git clone <repository-url>
cd claude-build-analyzer
```

2. **Build both servers**:
```bash
# Build both servers using Makefile
make build

# Or build individually:
make build-analyzer      # BigQuery-focused server
make ocp-metadata-server  # Metadata-focused server

# Alternatively, build manually:
go build -o build-analyzer cmd/build-analyzer/main.go
go build -o ocp-metadata-server cmd/ocp-metadata-server/main.go
```

3. **Set up Google Cloud credentials** (for build-analyzer only):
```bash
# Place your service account key
mkdir -p .credentials
cp /path/to/your/google-service-account.json .credentials/google.json

# Or use gcloud CLI
gcloud auth application-default login
```

## Configuration

### Tri-Server MCP Configuration

Add all three servers to your Claude Code MCP configuration:

```json
{
  "mcpServers": {
    "build-analyzer": {
      "command": "/path/to/claude-build-analyzer/build-analyzer",
      "env": {
        "GOOGLE_APPLICATION_CREDENTIALS": "/path/to/claude-build-analyzer/.credentials/google.json"
      }
    },
    "ocp-metadata": {
      "command": "/path/to/claude-build-analyzer/ocp-metadata-server"
    },
    "jenkins-server": {
      "command": "/path/to/claude-build-analyzer/jenkins-server",
      "env": {
        "JENKINS_USERNAME": "your-jenkins-username",
        "JENKINS_TOKEN": "your-jenkins-api-token"
      }
    }
  }
}
```

### Environment Variables

**Build Analyzer Server**:
- `GOOGLE_APPLICATION_CREDENTIALS`: Path to Google Cloud service account key
- `BQ_PROJECT_ID`: BigQuery project ID (default: `openshift-art`)

**OCP Metadata Server**:
- No external credentials required (uses public git repositories)

## Usage

### Core Workflow: "Why did ironic fail in 4.21?"

1. **Component Identification**: 
   ```
   Claude → ocp-metadata:search_components("ironic", "4.21")
   → Returns: ["ironic", "ironic-agent", "ironic-static-ip-manager"]
   ```

2. **Build Failure Analysis**:
   ```
   Claude → build-analyzer:query_build_failures(["ironic"], group="4.21") 
   → Returns: Recent failures with architecture breakdown
   ```

3. **Configuration Analysis**:
   ```
   Claude → ocp-metadata:get_component_metadata("ironic", "4.21")
   → Returns: Component configuration and dependencies
   ```

4. **Log Analysis**:
   ```
   Claude → build-analyzer:analyze_build_logs("ironic")
   → Returns: Konflux build URLs and failure patterns
   ```

5. **Root Cause Correlation**: Claude correlates metadata + build data for evidence-based diagnosis

**For complete analysis workflows and investigation patterns, see [docs/ANALYSIS_METHODOLOGY.md](docs/ANALYSIS_METHODOLOGY.md)**

## Available Tools

### Build Analyzer Server Tools
- **`query_build_failures`**: Analyze recent build failures across components and architectures
- **`analyze_build_logs`**: Access detailed build logs and extract failure patterns  
- **`compare_builds`**: Compare successful vs failed builds to identify differences

### OCP Metadata Server Tools
- **`get_component_metadata`**: Retrieve component configurations from ocp-build-data
- **`search_components`**: Find components using fuzzy matching for partial names
- **`search_by_field`**: Search components by YAML field values with dot-notation

### Jenkins Server Tools
- **`query_jenkins_builds`**: Query Konflux builds from Jenkins with filtering
- **`analyze_jenkins_logs`**: Retrieve Jenkins console logs and extract build parameters
- **`correlate_jenkins_builds`**: Correlate Jenkins execution with BigQuery build records

**Process Focus**: Use `GetToolList()` on any server to discover all available methods and parameters dynamically.

**For detailed tool usage and analysis workflows, see [docs/ANALYSIS_METHODOLOGY.md](docs/ANALYSIS_METHODOLOGY.md)**

## Data Sources

### BigQuery Database (Build Analyzer)
- **Project**: `openshift-art`
- **Dataset**: `events` 
- **Table**: `builds`
- **Performance**: Partitioned queries with 1-5 second response times
- **Data**: Build records, outcomes, logs, metadata for all architectures

### OCP Build Data Repository (Metadata Server)
- **Source**: `https://github.com/openshift-eng/ocp-build-data`
- **Branches**: OpenShift 4.12 through 4.21 (automatically managed)
- **Local Storage**: `~/.claude-build-analyzer/ocp-build-data/versions/`
- **Content**: Component configurations, dependencies, hermetic settings
- **Updates**: Automatic git fetch for latest metadata

## Performance Features

### Concurrent Operations
- **Parallel BigQuery queries**: Multiple components processed simultaneously
- **Async metadata loading**: Repository initialization in background
- **Goroutine-based**: Efficient resource utilization across both servers

### Intelligent Caching
- **Metadata caching**: Component configurations cached per branch
- **Git repository management**: Local clones with automatic updates
- **Query optimization**: BigQuery connection pooling and partition filtering

### Memory Efficiency
- **Dual binary deployment**: Each server optimized for its purpose
- **Shared abstractions**: Common code patterns reduce duplication
- **Resource cleanup**: Automatic cleanup of unused resources

## Project Structure

```
claude-build-analyzer/
├── cmd/
│   ├── build-analyzer/main.go         # Build analyzer server entry
│   ├── ocp-metadata-server/main.go    # Metadata server entry
│   └── jenkins-server/main.go         # Jenkins server entry
├── internal/
│   ├── bigquery/client.go             # BigQuery operations
│   ├── git/manager.go                 # Git repository management
│   ├── jenkins/client.go              # Jenkins API operations
│   ├── build-analyzer-server/server.go # Build analyzer MCP server
│   ├── metadata-server/server.go      # Metadata MCP server
│   └── jenkins-server/server.go       # Jenkins MCP server
├── pkg/shared/                        # Shared abstractions
│   ├── mcp.go                         # Common MCP protocol structures
│   ├── git.go                         # Git interfaces and data types
│   ├── yaml.go                        # Flexible YAML parsing utilities
│   └── server.go                      # Base MCP server implementation
├── docs/                              # Updated tri-server documentation
├── .mcp.json                          # Tri-server MCP configuration
├── build-analyzer                     # BigQuery-focused binary
├── ocp-metadata-server               # Metadata-focused binary
└── jenkins-server                     # Jenkins-focused binary
```

## Development

### Building from Source
```bash
# Build both servers
make build

# Or build individually
make build-analyzer      # BigQuery-focused server
make ocp-metadata-server  # Metadata-focused server

# Or manually:
go build -o build-analyzer cmd/build-analyzer/main.go
go build -o ocp-metadata-server cmd/ocp-metadata-server/main.go
```

### Testing Individual Servers

**Using Makefile (recommended)**:
```bash
make test-build-analyzer  # Test BigQuery server
make test-ocp-metadata    # Test metadata server
```

**Manual testing**:

**Test Build Analyzer**:
```bash
echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}' | GOOGLE_APPLICATION_CREDENTIALS=./.credentials/google.json ./build-analyzer
```

**Test Metadata Server**:
```bash
echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}' | ./ocp-metadata-server
```

### Adding New Tools

**For build analysis tools**: Add to `internal/mcp/server.go`
**For metadata tools**: Add to `internal/metadata-server/server.go`
**For shared functionality**: Add to `pkg/shared/`

## Authentication

### Google Cloud Setup (Build Analyzer Only)

1. **Create a service account** in the `openshift-art` project
2. **Grant BigQuery permissions**:
   - BigQuery Data Viewer
   - BigQuery Job User  
3. **Download the service account key** as JSON
4. **Place in `.credentials/google.json`**

### Repository Access (Metadata Server)

No authentication required - uses public read-only access to ocp-build-data.

## Troubleshooting

### Build Analyzer Issues

**BigQuery authentication errors**:
- Verify `.credentials/google.json` is valid
- Check service account permissions in GCP console
- Test with: `gcloud auth application-default print-access-token`

**Partition elimination errors**:
- All BigQuery queries include required `start_time` filters
- Check query logs for partition filter validation

### Metadata Server Issues

**Git repository errors**:
- Check internet connectivity to GitHub
- Verify disk space for multiple repositories (~500MB total)
- Check permissions in `~/.claude-build-analyzer/` directory

**YAML parsing warnings**:
- YAML warnings are expected for complex structures
- Server uses flexible parsing with fallback patterns
- Component data is still extracted successfully

### General MCP Issues

**Connection failures**:
- Verify Claude Code MCP configuration syntax
- Check that binary paths are absolute
- Ensure binaries have execute permissions: `chmod +x build-analyzer ocp-metadata-server`

**Performance issues**:
- Check available memory (recommended: 2GB+ RAM)
- Monitor disk space for git repositories
- Verify network connectivity to BigQuery and GitHub

## Performance Characteristics

### Build Analyzer Server
- **Startup time**: ~5-10 seconds (BigQuery client initialization)
- **Query response**: ~1-5 seconds for build failure analysis
- **Memory usage**: ~50-200MB for query operations
- **Concurrent queries**: Up to 10 simultaneous BigQuery operations

### OCP Metadata Server  
- **Startup time**: ~10-30 seconds (git repository initialization)
- **Query response**: ~0.5-2 seconds for metadata lookup
- **Memory usage**: ~100-300MB for cached metadata
- **Supported load**: Hundreds of metadata queries per hour

## Supported OpenShift Versions

- **OpenShift 4.21** (current development)
- **OpenShift 4.20** (current stable)
- **OpenShift 4.19, 4.18, 4.17, 4.16** (supported releases)
- **OpenShift 4.15, 4.14, 4.13, 4.12** (older releases)

New versions are automatically supported by adding branch names to the git manager configuration.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Implement changes following the dual server architecture
4. Add tests for new functionality
5. Update documentation for any API changes
6. Submit a pull request

### Understanding the Analysis Approach
Before contributing analysis features, review [docs/ANALYSIS_METHODOLOGY.md](docs/ANALYSIS_METHODOLOGY.md) to understand the evidence-based analysis principles and systematic investigation workflows.

### Code Guidelines
- Follow Go conventions and `gofmt` formatting
- Use shared abstractions in `pkg/shared/` for common functionality
- Add logging for debugging complex operations
- Maintain backward compatibility with existing MCP tools
- Keep servers focused on their specific responsibilities

## License

MIT License - see LICENSE file for details.