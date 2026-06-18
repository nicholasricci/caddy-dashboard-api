# Glossary — Caddy Dashboard

## UpstreamProfile

Admin-defined standard registration for a Caddy proxy discovery group: a named set of bindings that map Caddy `@id` handlers to ports on the registering instance.

## DomainProfile

Admin-defined standard for M2M hostname registration on a Caddy proxy discovery group: bindings map Caddy `@id` handlers (and optional `match_indexes`) where hostnames are added at registration time.

## Binding

One entry in an UpstreamProfile: a Caddy `config_id` (`@id`) and the port used to build the upstream dial (`instance_ip:port`) when an instance registers.

## Domain binding

One entry in a DomainProfile: a Caddy `config_id` (`@id`) and optional `match_indexes` (default `[0]`) identifying which `match[]` host list to update when domains are registered.

## Registrazione per profilo

Machine-to-machine registration that applies all bindings in an UpstreamProfile or DomainProfile in a single atomic mutate-and-propagate operation.

## Discovery group (proxy)

The `DiscoveryConfig` whose nodes are Caddy reverse-proxy instances that receive upstream and domain mutations. Distinct from the application instance's own discovery/ASG.

## API key (M2M)

Scoped credential for automated calls. For profile-based registration, the key must list allowed profile IDs (upstream and/or domain) and the parent discovery group IDs.
