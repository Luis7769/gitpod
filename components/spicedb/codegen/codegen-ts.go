// Copyright (c) 2023 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License.AGPL.txt in the project root for license information.

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/authzed/spicedb/pkg/namespace"
	"github.com/authzed/spicedb/pkg/schemadsl/compiler"

	corev1 "github.com/authzed/spicedb/pkg/proto/core/v1"
	implv1 "github.com/authzed/spicedb/pkg/proto/impl/v1"
)

// codegenTypescript represents the codegenTypescript command
var codegenTypescript = &cobra.Command{
	Use:     "codegen ts",
	Aliases: []string{"ts"},
	Short:   "Generates SpiceDB API code for TypeScript",
	Run: func(cmd *cobra.Command, args []string) {
		schema := GetCompiledSchema()
		fmt.Print(generateTypeScriptDefinitions(schema))
	},
}

func generateTypeScriptDefinitions(schema *compiler.CompiledSchema) string {

	resource := "export type ResourceType ="
	resourceTypes := "export const AllResourceTypes: ResourceType[] = ["
	relation := "export type Relation ="
	permission := "export type Permission ="
	other := ""
	fluentApi := ""

	// list definitions
	for _, def := range schema.ObjectDefinitions {
		// make sure the first character is upper case
		simpleName := firstUpper(def.Name)
		resourceTypeName := simpleName + "ResourceType"
		resource += "\n  | " + resourceTypeName + ""
		resourceTypes += "\n  \"" + def.Name + "\","
		other += "\nexport type " + resourceTypeName + " = \"" + def.Name + "\";\n"
		fluentApi += "\n" + generateTsFluentAPI(def)
		// check if relations exists
		hasRelations := false
		for _, rel := range def.Relation {
			if namespace.GetRelationKind(rel) == implv1.RelationMetadata_RELATION {
				hasRelations = true
				break
			}
		}
		if hasRelations {
			relation += "\n  | " + simpleName + "Relation"
			other += "\nexport type " + simpleName + "Relation ="
			for _, rel := range def.Relation {
				if namespace.GetRelationKind(rel) == implv1.RelationMetadata_RELATION {
					other += "\n     | \"" + rel.Name + "\""
				}
			}
			other += ";\n"
		}
		// check if permissions exists
		hasPermissions := false
		for _, rel := range def.Relation {
			if namespace.GetRelationKind(rel) == implv1.RelationMetadata_PERMISSION {
				hasPermissions = true
				break
			}
		}
		if hasPermissions {
			permission += "\n  | " + simpleName + "Permission"
			// permissions
			other += "\nexport type " + simpleName + "Permission ="
			for _, rel := range def.Relation {
				if namespace.GetRelationKind(rel) == implv1.RelationMetadata_PERMISSION {
					other += "\n     | \"" + rel.Name + "\""
				}
			}
			other += ";\n"
		}
	}

	return `
		/**
		* Copyright (c) 2023 Gitpod GmbH. All rights reserved.
		* Licensed under the GNU Affero General Public License (AGPL).
		* See License.AGPL.txt in the project root for license information.
		*/

		// This file is generated by the spicedb/codegen/codegen.go. Do not edit manually.

		import { v1 } from "@authzed/authzed-node";

		export const InstallationID = "1";

		/**
		* We need to call our internal API with system permissions in some cases.
		* As we don't have other ways to represent that (e.g. ServiceAccounts), we use this magic constant to designated it.
		*/
		export const SYSTEM_USER = "SYSTEM_USER";

		` + resource + `;

		` + resourceTypes + `
		];

		` + relation + `;

		` + permission + `;

		` + other + `

		export const rel = {
			` + fluentApi + `
		};
		`
}

func generateTsFluentAPI(def *corev1.NamespaceDefinition) string {
	ifElse := func(cond bool, ifTrue string, ifFalse string) string {
		if cond {
			return ifTrue
		}
		return ifFalse
	}

	relations := ""
	for _, rel := range def.Relation {
		if namespace.GetRelationKind(rel) != implv1.RelationMetadata_RELATION {
			continue
		}

		objects := ``
		for _, t := range rel.TypeInformation.AllowedDirectRelations {
			if t.GetPublicWildcard() != nil {
				objects += `get any` + firstUpper(t.Namespace) + `() {
					return {
						...result2,
						subject: {
							object: {
								objectType: "` + t.Namespace + `",
								objectId: "*",
							},

						},
					} as v1.Relationship;
				},`
			} else {
				hasRelation := t.GetRelation() != "..."
				name := ifElse(hasRelation, t.Namespace+"_"+t.GetRelation(), t.Namespace)
				objects += `
				` + ifElse(t.Namespace == "installation",
					"get "+name+"()",
					name+"(objectId: string)") + ` {
					return {
						...result2,
						subject: {
							object: {
								objectType: "` + t.Namespace + `",
								objectId: ` + ifElse(t.Namespace == "installation", `InstallationID`, "objectId") + `,
							},
							` + ifElse(hasRelation, `optionalRelation: "`+t.GetRelation()+`",`, "") + `
						},
					} as v1.Relationship;
				},`
			}
		}

		relations += `
			get ` + rel.Name + `() {
				const result2 = {
                    ...result,
                    relation: "` + rel.Name + `",
                };
				return {
					` + objects + `
				};
			},

		`
	}

	return `
	` + ifElse(def.Name == "installation",
		`get `+def.Name+`() {`,
		``+def.Name+`(id: string) {`) + `
		const result: Partial<v1.Relationship> = {
			resource: {
				objectType: "` + def.Name + `",
				objectId: ` + ifElse(def.Name == "installation", `InstallationID`, "id") + `
			},
		};
		return {
			` + relations + `
		};
	},
`
}

func init() {
	rootCmd.AddCommand(codegenTypescript)
}
