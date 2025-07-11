#!/bin/bash

# run_rinha.sh - Script para executar a Rinha de Backend 2025

set -e  # Para em caso de erro

echo "🚀 Iniciando Rinha de Backend 2025..."

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Função para log colorido
log() {
    echo -e "${GREEN}[$(date +'%H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%H:%M:%S')] $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%H:%M:%S')] $1${NC}"
}

# 1. Limpar containers anteriores
log "🧹 Limpando containers anteriores..."
cd ../projeto
docker-compose down --remove-orphans || true
docker system prune -f || true

# 2. Build e start dos containers
log "🔨 Fazendo build dos containers..."
docker-compose build --no-cache

log "🚀 Iniciando containers..."
docker-compose up -d

# 3. Aguardar containers ficarem prontos
log "⏳ Aguardando containers ficarem prontos..."
sleep 10

# 4. Verificar se containers estão rodando
log "🔍 Verificando status dos containers..."
docker-compose ps

# 5. Verificar logs iniciais
log "📋 Verificando logs iniciais..."
docker-compose logs --tail=20

# 6. Testar se API está respondendo
log "🩺 Testando health check..."
for i in {1..30}; do
    if curl -s http://localhost:9999/health > /dev/null; then
        log "✅ API está respondendo!"
        break
    else
        warn "Tentativa $i/30 - API ainda não está pronta..."
        sleep 2
    fi
done

# 7. Executar testes k6
log "🧪 Executando testes k6..."
cd ../rinha-de-backend-2025/rinha-test

# Verificar se k6 está instalado
if ! command -v k6 &> /dev/null; then
    warn "k6 não encontrado, usando Docker..."
    docker run --rm -i --network host grafana/k6:latest run - < rinha.js
else
    log "Executando k6 localmente..."
    k6 run rinha.js
fi

# 8. Mostrar logs finais
log "📊 Logs finais dos containers..."
cd ../../projeto
docker-compose logs --tail=50

log "🎉 Teste concluído!"
log "📈 Para ver métricas detalhadas: curl http://localhost:9999/payments-summary"
log "🛑 Para parar containers: docker-compose down"

echo ""
echo -e "${BLUE}=== COMANDOS ÚTEIS ===${NC}"
echo "Ver logs em tempo real: docker-compose logs -f"
echo "Reiniciar containers: docker-compose restart"
echo "Parar tudo: docker-compose down"
echo "Ver status: docker-compose ps"
