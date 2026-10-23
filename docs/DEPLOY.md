# Deploying Bloom to Tower

Pushes to `main` that touch `server/` or `docker-compose.yml` redeploy Bloom on
the Unraid server through a self-hosted GitHub Actions runner.

## One-time runner setup (Unraid)

1. In GitHub: repo **Settings > Actions > Runners > New self-hosted runner**,
   choose Linux x64, and copy the registration token it shows (it expires in
   about an hour).
2. In the Unraid terminal:

```
mkdir -p /tmp/runner-work
docker run -d --name bloom-runner --restart unless-stopped \
  -e REPO_URL=https://github.com/kvmediapp/bloom \
  -e RUNNER_TOKEN=PASTE_TOKEN_HERE \
  -e RUNNER_NAME=tower \
  -e LABELS=bloom-tower \
  -e RUNNER_WORKDIR=/tmp/runner-work \
  -v /tmp/runner-work:/tmp/runner-work \
  -v /var/run/docker.sock:/var/run/docker.sock \
  myoung34/github-runner:latest
```

3. The runner should show as Idle in the GitHub runners list.

Security: keep the repo private, or at minimum under
**Settings > Actions > General > Fork pull request workflows**, require
approval for all outside contributors. The deploy workflow only runs on pushes
to `main`.
