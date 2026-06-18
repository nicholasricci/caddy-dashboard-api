# Glossary — Caddy Dashboard

## UpstreamProfile

Admin-defined standard registration for a Caddy proxy discovery group: a named set of bindings that map Caddy `@id` handlers to ports on the registering instance.

## Binding

One entry in an UpstreamProfile: a Caddy `config_id` (`@id`) and the port used to build the upstream dial (`instance_ip:port`) when an instance registers.

## Registrazione per profilo

Machine-to-machine registration that applies all bindings in an UpstreamProfile in a single atomic mutate-and-propagate operation, given the instance private IP.

## Discovery group (proxy)

The `DiscoveryConfig` whose nodes are Caddy reverse-proxy instances that receive upstream mutations. Distinct from the application instance's own discovery/ASG.

## API key (M2M)

Scoped credential for automated calls. For profile-based registration, the key must list allowed upstream profile IDs and the parent discovery group IDs.
