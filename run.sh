go build

if [ $? -eq 0 ]
then
    echo "Build ok"
    ./insights-operator-mock
else
    echo "Build failed"
fi
