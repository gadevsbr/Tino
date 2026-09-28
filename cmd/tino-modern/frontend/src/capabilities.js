export function normalizeCapabilities(value) {
  return {
    modules: value?.modules ?? value?.Modules ?? [],
    roles: value?.roles ?? value?.Roles ?? [],
    workspaceRoot: value?.workspaceRoot ?? value?.WorkspaceRoot ?? '',
  }
}
