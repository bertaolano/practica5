#!/bin/bash

cd cliente
echo "Inicia compiación cliente"
CGO_ENABLED=0 go build -o . ./...
echo "Compilación finalizada cliente"
cd ..
cd srvraft
echo "Inicia compiación servidor"
CGO_ENABLED=0 go build -o . ./...
echo "Compilación finalizada srvraft"
cd ..


#Eliminar cluster
echo "Elimina cluster"
kind delete cluster
echo "Cluster eliminado"

#Parar y eliminar registry
echo "Para y elimina registry"
docker stop kind-registry
docker rm kind-registry
echo "Registry eliminado"

#Arrancar el cluster
./kind-with-registry.sh

#Construir imagen srvraft
cd srvraft
echo "Construir imagen srvraft"
docker build . -t localhost:5001/srvraft:latest
docker push localhost:5001/srvraft:latest
cd ..

#Construir imagen cliente
cd cliente
echo "Construir imagen cliente"
docker build . -t localhost:5001/cliente:latest
docker push localhost:5001/cliente:latest
cd ..

#Ejecutar script raft
echo "Elecutar el script de raft"
./raft.sh