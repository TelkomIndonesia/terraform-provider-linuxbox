# linux_script

Manage arbritrary resource by specifying scripts that will be executed remotely on Create|Read|Update|Delete phase.

## Example Usage

```hcl
resource "linux_script" "script" {
    lifecycle_commands {
        create = "apt install -y $PACKAGE_NAME=$PACKAGE_VERSION"
        read = "apt-cache policy $PACKAGE_NAME | grep 'Installed:' | grep -v '(none)' | awk '{ print $2 }' | xargs | tr -d '\n'"
        update = "apt install -y $PACKAGE_NAME=$PACKAGE_VERSION"
        delete = "apt remove -y $PACKAGE_NAME"
    }
    environment = {
        PACKAGE_NAME = "apache2"
        PACKAGE_VERSION = "2.4.18-2ubuntu3.4"
    }
}
```

## Argument Reference

The following arguments are supported:

- `lifecycle_commands` - (Required) Block that contains commands to be remotely executed respectively in Create|Read|Update|Delete phase. For complex commands, use [the file function](https://www.terraform.io/docs/configuration/functions/file.html).
- `triggers` - (Optional, string map) Attribute that will trigger resource recreation on changes just like the one in [null_resource](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource#triggers). Default empty map.
- `environment` - (Optional, string map ) A list of linux environment that will be available in each `lifecycle_commands`. Default empty map.
- `sensitive_environment` - (Optional, string map) Just like `environment` except they don't show up in log files. Default empty map.
- `interpreter` - (Optional, string list) Interpreter for running each `lifecycle_commands`. Default empty list which is equal to `[ "sh" ,  "-c" ]`.
- `working_directory` - (Optional, string) The working directory where each `lifecycle_commands` is executed. Default empty string.

### lifecycle_commands

The following arguments are supported:

- `create` - (Required, string) Commands that will be execued in Create phase.
- `read` - (Required, string) Commands that will be execued in Read phase and after execution of `create` or `update` commands. Terraform will record the output of these commands and trigger update/recreation when the output changes. If the result of running these commands is empty string, the resource is considered as destroyed.
- `update` - (Optional, string) Commands that will be execued in Update phase. Omiting this will disable Update phase and trigger resource recreation (Delete -> Create) each time terraform detect changes.
- `delete` - (Required, string) Commands that will be execued in Delete phase.

## Attribute Reference

- `output` - (string) The raw output of `read` commands.