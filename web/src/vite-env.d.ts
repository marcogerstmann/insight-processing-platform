/// <reference types="vite/client" />

// Typed access to the VITE_-prefixed vars from web/.env (see .env.example).
interface ImportMetaEnv {
  readonly VITE_AWS_REGION: string;
  readonly VITE_COGNITO_USER_POOL_ID: string;
  readonly VITE_COGNITO_CLIENT_ID: string;
  readonly VITE_REST_API_ENDPOINT: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
