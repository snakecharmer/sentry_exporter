## Configuration

Sentry exporter use a configuration file to define the entrypoint to the sentry installation.

For example if you are using the standard sentry.io use the following template and change the organization_slug & token:

```yml
modules:
  sentry:
    prober: http
    timeout: 5s
    http:
      prefix: https://sentry.io/api/0/projects/{organization_slug}
      headers:
        Authorization: Bearer {Token}
```
