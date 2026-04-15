/** Minimal Swagger 2.0 shapes for listing operations */
export type SwaggerOperation = {
  summary?: string;
  tags?: string[];
};

export type SwaggerPathItem = {
  get?: SwaggerOperation;
  post?: SwaggerOperation;
  put?: SwaggerOperation;
  patch?: SwaggerOperation;
  delete?: SwaggerOperation;
  options?: SwaggerOperation;
  head?: SwaggerOperation;
};

export type SwaggerDocument = {
  swagger?: string;
  openapi?: string;
  paths?: Record<string, SwaggerPathItem>;
  info?: { title?: string; version?: string };
};
