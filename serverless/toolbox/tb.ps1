# Запускає команду (make/aws/terraform/go) у Linux-контейнері lambda-toolbox,
# приєднаному до мережі Floci. Використання з папки serverless:
#   .\toolbox\tb.ps1 "make test"
#   .\toolbox\tb.ps1 "make smoke"
# Перший запуск сам збудує образ (кілька хвилин).
param([Parameter(Mandatory = $true, ValueFromRemainingArguments = $true)][string[]]$Cmd)

$root = Split-Path -Parent $PSScriptRoot
docker image inspect lambda-toolbox *> $null
if ($LASTEXITCODE -ne 0) { docker build -t lambda-toolbox "$PSScriptRoot" }

# Мережа serverless_default з'являється після `docker compose up -d`;
# для make test / make build вона не потрібна.
$net = @()
docker network inspect serverless_default *> $null
if ($LASTEXITCODE -eq 0) { $net = @('--network', 'serverless_default') }

docker run --rm @net `
  -v "${root}:/work" -v gocache:/go/pkg -w /work `
  -v /var/run/docker.sock:/var/run/docker.sock `
  -e AWS_ENDPOINT_URL=http://floci:4566 -e TF_VAR_floci_endpoint=http://floci:4566 `
  lambda-toolbox bash -c ($Cmd -join ' ')
