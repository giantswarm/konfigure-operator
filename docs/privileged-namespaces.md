# Privileged namespaces

The operator renders configuration with the platform SOPS keys and can write ConfigMaps and Secrets
in every namespace. It therefore only reconciles Konfigurations in its privileged namespaces, by
default `giantswarm` and `flux-giantswarm`.

## Konfigurations outside the privileged namespaces

A Konfiguration in any other namespace is not reconciled: the operator does not load the SOPS keys,
fetch the source or schema, or write anything for it. Its `Ready` condition is set to `False` with
reason `NamespaceNotAllowed`, and the message lists the privileged namespaces. It is not retried on
a timer.

ConfigMaps and Secrets such a Konfiguration rendered before the operator started refusing it are
left in place. Delete them by hand, and rotate any secret values they hold if people who should not
see them could read that namespace.

Konfigurations in a privileged namespace may reference a GitRepository, KonfigurationSchema and
destination in any namespace.

## Reference names

The names and namespaces of the GitRepository and KonfigurationSchema references and the
destination namespace must be valid DNS-1123 names. The CRD validates this, and the operator checks
it again for every Konfiguration, so it holds where the CRD validation is not installed yet. A
Konfiguration that fails the check gets reason `InvalidReference` on its `Ready` condition.

## Configuring privileged namespaces

Set the namespaces with the `privilegedNamespaces` Helm value, which becomes the
`--privileged-namespaces` flag:

```yaml
privilegedNamespaces:
  - giantswarm
  - flux-giantswarm
```

Only list namespaces where everyone who can create a Konfiguration may also read every Secret the
operator can decrypt. With an empty list, every Konfiguration is refused.

The operator does not start with an invalid privileged namespace.

## Upgrading

The CRD now validates the reference names and namespaces. On Kubernetes versions without CRD
validation ratcheting (before 1.30), an existing Konfiguration with an invalid value can no longer
be updated, which also blocks removing the operator's finalizer when it is deleted. Such a
Konfiguration could never have rendered anything; find and delete these objects before rolling out
the new CRD. Until then the operator cannot record the `InvalidReference` refusal on them and only
logs it.
