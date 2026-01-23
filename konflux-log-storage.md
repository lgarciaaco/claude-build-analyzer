# Konflux Build Log Storage Architecture

## Overview

This document describes where and how Konflux build logs are stored when builds fail, based on the art-tools KonfluxWatcher implementation.

## Primary Storage Locations

### 1. In-Memory Cache (Runtime)

**Location**: `KonfluxWatcher._pod_cache`
```python
# konflux_watcher.py:75-77
self._pod_cache: Dict[str, Dict[str, Tuple[Dict, Dict[str, str]]]] = {}
# Structure: pipelinerun_name -> {pod_name -> (pod_dict, container_logs)}
```

**Characteristics**:
- **Thread Safe**: Protected by `self._cache_lock` (RLock)
- **Automatic Collection**: Polls every 60 seconds
- **Merge Strategy**: New logs merged with existing, preventing loss
- **Lifetime**: Persists until PipelineRun reaches terminal state

### 2. Database Records (Persistent)

**Location**: TaskRun records in build database
```python
# konflux_image_builder.py:847-857
container_info = {
    'name': container.name,
    'log_output': container.get_log_content(),  # ← LOGS STORED HERE
    'exit_code': exit_code,
    'state': state,
    # ... other metadata
}
```

**Integration**: Container logs become part of structured TaskRun records for persistent storage.

## Log Collection Process

### Automatic Collection Criteria

**Trigger Conditions** (konflux_watcher.py:207-239):
- Pod phase **NOT** in `["Succeeded", "Pending", "Running"]`
- Container status: `container.is_failed == True`
- Applies to both init containers and regular containers

**Collection Method**:
```python
# Direct Kubernetes API call
log_content = self.corev1_client.read_namespaced_pod_log(
    name=pod_name,
    namespace=self.namespace,
    container=container.name,
    _request_timeout=self.request_timeout,
)
```

### What Gets Collected

✅ **Collected**:
- Failed container logs (complete content)
- Both init and regular containers
- Error output and standard output

❌ **Not Collected**:
- Logs from successful containers
- Logs from pending/running pods
- Logs from containers that haven't failed

## Access Patterns

### Programmatic Access

```python
# Get PipelineRun with logs
pipelinerun_info = await watcher.get_pipelinerun_info("my-pipeline-run")

# Access container logs
for pod_name, pod_info in pipelinerun_info.pods.items():
    for container in pod_info.get_all_containers():
        if container.is_failed():
            log_content = container.get_log_content()
            # or
            log_content = pod_info.get_log_content(container.name)
```

### UI Access

**Konflux UI URLs**: Each build record stores `task_url` for manual log viewing
```
https://konflux-ui.apps.kflux-ocp-p01.7ayg.p1.openshiftapps.com/ns/ocp-art-tenant/applications/{application}/pipelineruns/{name}
```

## Key Implementation Files

- **`konflux_watcher.py`**: Main log collection and caching logic
- **`pipelinerun_utils.py`**: Data structures (PodInfo, ContainerInfo) and log access methods
- **`konflux_image_builder.py`**: Database integration for TaskRun records
- **`konflux_client.py`**: UI URL generation

## Storage Lifecycle

1. **Detection**: KonfluxWatcher detects failed containers during 60-second polling
2. **Collection**: Logs fetched via Kubernetes API for failed containers
3. **Cache**: Stored in memory cache with merge strategy
4. **Database**: Logs included in TaskRun records for persistence
5. **Cleanup**: Memory cache cleaned when PipelineRuns reach terminal state

## Important Notes

- **Memory vs Persistence**: In-memory cache is temporary, database records provide persistence
- **Failed Containers Only**: Only failed containers have logs collected automatically
- **Thread Safety**: All cache operations are thread-safe via locking
- **Merge Strategy**: Logs from multiple polling cycles are merged, preventing data loss
- **Process Lifetime**: In-memory logs lost when KonfluxWatcher process terminates

## Usage Context

This information is primarily relevant for:
- Debugging failed Konflux builds
- Understanding where to find error logs when hermetic conversions fail
- Implementing tooling that needs to access build failure logs
- JIRA ticket creation for hermetic build failures (referencing specific container logs)