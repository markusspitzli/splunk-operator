#!/usr/bin/env bash

if [ "${INSTALL_OPERATOR}" = true ]; then
    echo "installing operator"
    helm upgrade --install splunk-operator ../../../../helm-chart/splunk-operator  --namespace $NAMESPACE --set splunkOperator.clusterWideAccess=false --set splunkOperator.image.repository=${KUTTL_SPLUNK_OPERATOR_IMG}
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "operator installation failed"
        echo "Error"
        exit $retVal
    else 
        kubectl wait --for=condition=ready pod -l control-plane=controller-manager --timeout=600s -n $NAMESPACE
        retVal=$?
        echo "operator installation success"
        exit $retVal
    fi
else 
    echo "INSTALL_OPERATOR env is not set, skip"
fi
