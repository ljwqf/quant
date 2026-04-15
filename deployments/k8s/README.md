# Kubernetes 部署说明

## 前提条件

- Kubernetes 集群 1.24+
- `kubectl` 已配置并连接到目标集群
- 容器镜像已构建并推送到镜像仓库

## 部署步骤

### 1. 构建并推送镜像

```bash
docker build -t okx-quant:latest .
docker tag okx-quant:latest <your-registry>/okx-quant:latest
docker push <your-registry>/okx-quant:latest
```

### 2. 更新配置

编辑 `secret.yaml`，填入实际密钥值（建议在生产环境使用外部密钥管理器）：

```bash
kubectl edit secret quant-secrets -n quant-trading
```

### 3. 部署

```bash
# 创建命名空间
kubectl apply -f deployments/k8s/namespace.yaml

# 等待命名空间就绪
kubectl wait --for=condition=Ready namespace/quant-trading --timeout=60s

# 部署所有资源
kubectl apply -f deployments/k8s/configmap.yaml
kubectl apply -f deployments/k8s/secret.yaml
kubectl apply -f deployments/k8s/pvc.yaml
kubectl apply -f deployments/k8s/deployment.yaml
kubectl apply -f deployments/k8s/service.yaml
```

### 4. 验证

```bash
# 检查 Pod 状态
kubectl get pods -n quant-trading

# 检查服务
kubectl get svc -n quant-trading

# 查看日志
kubectl logs -f deploy/quant-trader -n quant-trading

# 健康检查
kubectl port-forward svc/quant-trader 8765:8765 -n quant-trading &
curl http://localhost:8765/health
curl http://localhost:8765/ready
```

## 运维操作

### 查看指标

```bash
kubectl port-forward svc/quant-trader 8765:8765 -n quant-trading &
curl http://localhost:8765/metrics
```

### 重启服务

```bash
kubectl rollout restart deployment/quant-trader -n quant-trading
```

### 更新配置

```bash
kubectl edit configmap quant-config -n quant-trading
kubectl rollout restart deployment/quant-trader -n quant-trading
```

## 注意事项

- 交易系统使用 `Recreate` 策略，更新时会先停止旧 Pod 再启动新 Pod
- 副本数固定为 1，避免重复下单
- 敏感数据建议使用 Vault 或云厂商密钥管理服务
