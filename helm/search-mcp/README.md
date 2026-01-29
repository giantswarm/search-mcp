# search-mcp

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![AppVersion: 0.0.1](https://img.shields.io/badge/AppVersion-0.0.1-informational?style=flat-square)

MCP server for access to Giant Swarm documentation and more

**Homepage:** <https://github.com/giantswarm/search-mcp>

## Source Code

* <https://github.com/giantswarm/search-mcp>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| replicaCount | int | `2` | Number of pods to run |
| image | object | `{"pullPolicy":"IfNotPresent","registry":"gsoci.azurecr.io","repository":"giantswarm/search-mcp","tag":""}` | Container image settings |
| image.registry | string | `"gsoci.azurecr.io"` | Registry host name |
| image.repository | string | `"giantswarm/search-mcp"` | Repository name |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| image.tag | string | `""` | Optional: overrides the image tag. Default tag is identical with the chart appVersion. |
| imagePullSecrets | list | `[]` | Image pull secrets |
| nameOverride | string | `""` | Optional: overrides for the generated application name, but will be suffixed with the release name. |
| fullnameOverride | string | `""` | Optional: overrides the generated application name completely. Cannot be combined with nameOverride. |
| serviceAccount | object | `{"annotations":{},"automount":false,"create":true,"name":""}` | Service account settings |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.automount | bool | `false` | Automatically mount a ServiceAccount's API credentials? |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| podAnnotations | object | `{}` | Pod annotations |
| podLabels | object | `{}` | Pod labels |
| podSecurityContext | object | `{}` | Pod security context |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true,"runAsNonRoot":true,"runAsUser":1000,"seccompProfile":{"type":"RuntimeDefault"}}` | Container security context |
| service | object | `{"annotations":{},"labels":{},"port":80,"type":"ClusterIP"}` | Service settings |
| route | object | `{"additionalRules":[],"annotations":{},"enabled":true,"filters":[],"hostnames":[],"kind":"HTTPRoute","labels":{},"matches":[{"path":{"type":"PathPrefix","value":"/"}}],"name":"","parentRefs":[],"securityPolicy":{"annotations":{},"enabled":false,"labels":{}}}` | Gateway API route configuration. More information can be found at https://gateway-api.sigs.k8s.io/ |
| route.enabled | bool | `true` | Set to true to enable route creation |
| route.kind | string | `"HTTPRoute"` | Kind of route to create. |
| route.name | string | `""` | Override the route name (defaults to the name of the Helm release) |
| route.annotations | object | `{}` | Optional extra annotations for the route |
| route.labels | object | `{}` | Optional extra labels for the route |
| route.hostnames | list | `[]` | Hostnames that the route should match. Supports templating with {{ .Values.xxx }} |
| route.parentRefs | list | `[]` | Optional parent gateway references |
| route.matches | list | `[{"path":{"type":"PathPrefix","value":"/"}}]` | Request matching rules |
| resources | object | `{"limits":{"cpu":"100m","memory":"256Mi"},"requests":{"cpu":"20m","memory":"32Mi"}}` | Resource requests and limits |
| resources.requests | object | `{"cpu":"20m","memory":"32Mi"}` | Resource requests |
| resources.requests.cpu | string | `"20m"` | CPU requested |
| resources.requests.memory | string | `"32Mi"` | Memory requested |
| resources.limits | object | `{"cpu":"100m","memory":"256Mi"}` | Resource limits |
| resources.limits.cpu | string | `"100m"` | CPU limit |
| resources.limits.memory | string | `"256Mi"` | Memory limit |
| livenessProbe | object | `{"httpGet":{"path":"/healthz","port":"http"}}` | Liveness probe |
| livenessProbe.httpGet | object | `{"path":"/healthz","port":"http"}` | HTTP GET probe |
| livenessProbe.httpGet.path | string | `"/healthz"` | Path to access on the HTTP server |
| livenessProbe.httpGet.port | string | `"http"` | Port to access on the HTTP server. Can be a named port or a number in string format. |
| readinessProbe | object | `{"httpGet":{"path":"/healthz","port":"http"}}` | Readiness probe |
| readinessProbe.httpGet | object | `{"path":"/healthz","port":"http"}` | HTTP GET probe |
| readinessProbe.httpGet.path | string | `"/healthz"` | Path to access on the HTTP server |
| readinessProbe.httpGet.port | string | `"http"` | Port to access on the HTTP server. Can be a named port or a number in string format. |
| strategy | object | `{"rollingUpdate":{"maxSurge":1,"maxUnavailable":1},"type":"RollingUpdate"}` | Deployment strategy |
| strategy.rollingUpdate | object | `{"maxSurge":1,"maxUnavailable":1}` | Rolling update settings. Only applicable if type is RollingUpdate. |
| strategy.rollingUpdate.maxUnavailable | int | `1` | Maximum number of pods that can be unavailable during the update. |
| strategy.rollingUpdate.maxSurge | int | `1` | Maximum number of pods that can be created above the desired number of pods during the update. |
| volumes | list | `[]` | Extra volumes |
| volumeMounts | list | `[]` | Extra volume mounts |
| env | list | `[]` | Environment variables for the container Example for OAuth configuration:   - name: OAUTH_ISSUER_URL     value: "https://dex.example.com"   - name: OAUTH_CLIENT_ID     value: "searchmcp" |
| envFrom | list | `[]` | Environment variables from secrets or configmaps Example:   - secretRef:       name: search-mcp-oauth |
| nodeSelector | object | `{}` | Optional node selector for pod assignment |
| tolerations | list | `[]` | Optional tolerations for pod assignment |
| topologySpreadConstraints | list | `[{"labelSelector":{"matchLabels":{"app.kubernetes.io/instance":"{{ .Release.Name }}","app.kubernetes.io/name":"{{ include \"search-mcp.name\" . }}"}},"maxSkew":1,"topologyKey":"topology.kubernetes.io/zone","whenUnsatisfiable":"ScheduleAnyway"},{"labelSelector":{"matchLabels":{"app.kubernetes.io/instance":"{{ .Release.Name }}","app.kubernetes.io/name":"{{ include \"search-mcp.name\" . }}"}},"maxSkew":1,"topologyKey":"kubernetes.io/hostname","whenUnsatisfiable":"ScheduleAnyway"}]` | Pod topology spread constraints |
| podDisruptionBudget | object | `{"enabled":true,"minAvailable":1}` | Settings about PodDisruptionBudget creation |
| podDisruptionBudget.enabled | bool | `true` | Whether to create a PodDisruptionBudget resource |
| podDisruptionBudget.minAvailable | int | `1` | Number of pods that must be available after a disruption. Cannot be combined with maxUnavailable. |
| serviceMonitor | object | `{"enabled":true}` | Settings about ServiceMonitor creation for Prometheus Operator |
| serviceMonitor.enabled | bool | `true` | Whether to create a ServiceMonitor resource |
