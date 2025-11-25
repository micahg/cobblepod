/**
 * Runtime configuration loaded from auth.json
 * This is populated at container startup via envsubst from auth.template.json
 */

export interface RuntimeConfig {
  domain: string;
  clientId: string;
  audience: string;
  apiUrl: string;
}

let runtimeConfig: RuntimeConfig | undefined;

/**
 * Load runtime configuration from /auth.json
 * This should be called before the React app initializes
 */
export async function loadRuntimeConfig(): Promise<RuntimeConfig> {
  if (runtimeConfig) {
    return runtimeConfig;
  }

  try {
    // Create abort controller for timeout
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 5000); // 5 second timeout

    const response = await fetch('/auth.json', { signal: controller.signal });
    clearTimeout(timeoutId);
    
    if (!response.ok) {
      throw new Error(`Failed to load config: ${response.status}`);
    }
    runtimeConfig = await response.json();
    if (!runtimeConfig) {
      throw new Error('Config loaded but is null');
    }
    return runtimeConfig;
  } catch (error) {
    if (error instanceof Error && error.name === 'AbortError') {
      console.error('Timeout loading runtime configuration');
      throw new Error('Configuration load timeout - server may be unavailable');
    }
    console.error('Failed to load runtime configuration:', error);
    throw error;
  }
}

/**
 * Get the loaded runtime configuration
 * Throws if config hasn't been loaded yet
 */
export function getRuntimeConfig(): RuntimeConfig {
  if (!runtimeConfig) {
    throw new Error('Runtime config not loaded. Call loadRuntimeConfig() first.');
  }
  return runtimeConfig;
}

/**
 * Check if runtime config is loaded
 */
export function isConfigLoaded(): boolean {
  return runtimeConfig !== undefined;
}
