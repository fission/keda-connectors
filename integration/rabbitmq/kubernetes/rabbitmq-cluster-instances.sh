#count=$(kubectl get statefulset rabbitmq --namespace rabbits  -o go-template='{{.spec.replicas}}')
#i=0
kubectl apply -f rabbit-statefulset.yaml
count=$(kubectl get statefulset rabbitmq --namespace rabbits  -o go-template='{{.spec.replicas}}')
i=0
while [ $i -lt $count ] 
do

  kubectl wait pod rabbitmq-$i --for=condition=Ready --timeout=-1s -n rabbits
  res=$?
  echo $res
  i=`expr $i + 1`  
done

