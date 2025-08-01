module dynamic-pipeline // local module name—matches import prefix

go 1.21

replace pipeline-config => ../pipeline-config // path to sibling module

require pipeline-config v0.0.0-00010101000000-000000000000

require gopkg.in/yaml.v3 v3.0.1 // indirect
