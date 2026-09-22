# CONTEXT — hello (Wake-on-LAN)

Glossaire canonique v1.1. Pas de détails d'implémentation.

## Termes

- **device** : machine enregistrée à réveiller (nom, MAC, IP/broadcast).
- **MAC** : adresse MAC canonique d'un device, identifiant de réveil.
- **magic packet** : paquet de réveil envoyé à un device.
- **broadcast IP** : adresse de diffusion utilisée pour porter le magic packet.
- **reachability** : état observé d'un device : `up` / `down` / `unknown`.
- **last_seen** : dernier instant où un device a été observé `up`.
- **wake event** : tentative de réveil (manuelle ou planifiée), avec succès/échec.
- **trigger** : origine d'un wake event : `manual` ou `schedule`.
- **schedule / cron** : règle de réveil récurrente.
- **discovery scan** : exploration du LAN pour trouver des hôtes.
- **adopt** : enregistrement d'un hôte découvert comme device.
- **status change** : passage de `reachability` d'un device, diffusé en temps réel.

## Hors glossaire v1.1 (reporté v1.2)

Base de données, multi-utilisateur / rôles, TLS in-app. En v1.1 : persistance fichiers mono-instance, secret partagé unique durci, TLS au reverse-proxy.
