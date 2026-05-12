# trivia Helm chart

Self-hosted realtime trivia game: Vue SPA + Go backend + Postgres, fronted by
ingress-nginx with cert-manager-issued TLS.

## What ships

- **backend** Deployment (Go API + WebSocket `/ws`), Service `8080`
- **frontend** Deployment (nginx serving the SPA), Service `80`, with an
  override ConfigMap that strips the in-image `/api`+`/ws` proxy (the ingress
  routes those directly)
- **postgres** StatefulSet + PVC (toggle with `postgres.enabled`)
- **Ingress** for `trivia.oglimmer.com`, routing `/api`, `/ws`, `/health` to
  the backend and everything else to the frontend
- **SealedSecret** placeholder for `POSTGRES_PASSWORD`, `JWT_SECRET`,
  `ADMIN_PASSWORD`, optional `ANTHROPIC_API_KEY`

## Install

```bash
# 1. Seal the secret (one-time, against your cluster's controller):
kubectl create secret generic trivia-secret \
  --namespace default --dry-run=client -o yaml \
  --from-literal=POSTGRES_PASSWORD=$(openssl rand -hex 16) \
  --from-literal=JWT_SECRET=$(openssl rand -hex 32) \
  --from-literal=ADMIN_PASSWORD=<choose> \
  --from-literal=ANTHROPIC_API_KEY=<sk-ant-...> \
  | kubeseal --format yaml > templates/sealed-secret.yaml

# 2. Install
helm install trivia ./helm/trivia
```

## TLS / ingress

The default annotations target the `oglimmer-com-dns` cert-manager
ClusterIssuer (DNS-01 challenge against the `oglimmer.com` zone). The TLS
secret is `tls-trivia-ingress-dns`. Change `ingress.annotations.cert-manager.io/cluster-issuer`
and `ingress.tls[].secretName` to use a different issuer.

WebSocket upgrades work out-of-the-box with ingress-nginx; the annotations
disable response buffering and bump the read/send timeouts so the `/ws`
stream stays alive.

## Registry auth

Images live at `ghcr.io/oglimmer/trivia-{backend,frontend}`. If the GHCR
namespace is private, create a docker-registry pull secret and reference it:

```bash
kubectl create secret docker-registry ghcr-trivia \
  --docker-server=ghcr.io \
  --docker-username=<gh-user> \
  --docker-password=<gh-PAT-with-read:packages> \
  --docker-email=<email>
```

```yaml
# values.yaml override
imagePullSecrets:
  - name: ghcr-trivia
```
