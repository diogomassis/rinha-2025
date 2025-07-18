#!/bin/bash

set -e

echo "🚀 Iniciando solução da Rinha de Backend 2025..."

if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker não está rodando. Por favor, inicie o Docker primeiro."
    exit 1
fi

echo "📋 Verificando Payment Processors..."
if ! docker network ls | grep -q "payment-processor"; then
    echo "⚠️  Rede 'payment-processor' não encontrada."
    echo "   Por favor, suba os Payment Processors primeiro:"
    echo "   cd payment-processor && docker-compose up -d"
    exit 1
fi

echo "🔨 Fazendo build da aplicação..."
make all

echo "🐳 Criando imagem Docker..."
docker build -t rinha-2025 .

echo "🚀 Iniciando serviços..."
docker-compose up -d

echo "⏳ Aguardando serviços ficarem prontos..."
sleep 10

echo "🔍 Verificando status dos serviços..."
docker-compose ps

echo "🧪 Testando endpoints..."

echo "Testing POST /payments..."
response=$(curl -s -X POST http://localhost:9999/payments \
  -H "Content-Type: application/json" \
  -d '{"correlationId": "550e8400-e29b-41d4-a716-446655440000", "amount": 100.50}' \
  -w "%{http_code}")

echo "Response: $response"

echo "Testing GET /payments-summary..."
curl -s http://localhost:9999/payments-summary | jq .

echo "✅ Aplicação rodando com sucesso!"
echo "📊 Endpoints disponíveis:"
echo "   POST http://localhost:9999/payments"
echo "   GET  http://localhost:9999/payments-summary"
echo ""
echo "📝 Para ver logs:"
echo "   docker-compose logs -f app1 app2"
echo ""
echo "🛑 Para parar:"
echo "   docker-compose down"
