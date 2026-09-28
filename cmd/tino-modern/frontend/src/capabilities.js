export function normalizeCapabilities(value) {
  return {
    modules: value?.modules ?? value?.Modules ?? [],
    roles: value?.roles ?? value?.Roles ?? [],
    operators: value?.operators ?? value?.Operators ?? [],
    workspaceRoot: value?.workspaceRoot ?? value?.WorkspaceRoot ?? '',
  }
}
