

## Features

- **Feature01**: the CassKop special feature using the _unlockNextOperation which allows to recover when CassKop fails,
  must be treated with really good care as this parameter is removed when activated by local CassKop. I think We must
  prevent to let this parameter be set at CassandraMultiCluster level, and not to be removed from local CassandraCluster
  if it has been set up locally (remove from the difference detection)

- **Feature02**: Auto compute and update seedlist at MultiCassKop level

- **Feature03**: Specify the namespace we want to deploy onto for each kubernetes contexts

- **Feature04**: Allow to delete CassandraClusters when deleting CassandraMultiCluster
                 Make uses of a Finalizer to keep track of last CassandraMultiCluster before deleting

## Bugs

- **Bug01**: when changing parameter on a deployed cluster (for instance cassandra image), both clusters applied modification
  in the same time, this is not good, we need to only applied on one cluster and when OK apply to the next one
