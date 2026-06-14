# Runbook: broker stuck after a network partition

## Symptom

A broker pod is `Running` but is not part of the cluster: under-replicated partitions stay
non-zero, the broker isn't taking traffic, and producers/consumers see failures that don't
clear after connectivity is restored. Its logs repeat:

```
INFO [broker-2-to-controller-heartbeat-channel-manager] BrokerLifecycleManager
  [BrokerLifecycleManager id=2] Unable to send a heartbeat because the RPC got timed out
  before it could be sent.
INFO [group-coordinator-event-processor-2] CoordinatorRuntime$EventBasedCoordinatorTimer
  [GroupCoordinator id=2 topic=__consumer_offsets partition=11] The write event Timeout(...)
  ... timed out after 5000ms. Rescheduling it
```

## Cause

After a broker loses and then regains network connectivity, it can be left half-connected.
The process is up but its heartbeat channel to the KRaft controllers never re-establishes, so
it can't re-register or commit `__consumer_offsets` writes. It does **not** recover on its own.

## Resolution

Restart the stuck broker to force a clean rejoin:

```
kubectl delete pod my-cluster-broker-<id> -n <namespace>
```

The Strimzi operator recreates the pod immediately. It rejoins the quorum within ~1–2 minutes,
starts taking traffic again, and the cluster returns to full health.

## Verify recovery

- `kubectl get pods` — the broker is back `Running` and `Ready`.
- Broker dashboard: **brokers online** back to full count, **under-replicated partitions → 0**,
  **partitions under min ISR → 0**.
- Produce failures and messages lost back to **0**.
