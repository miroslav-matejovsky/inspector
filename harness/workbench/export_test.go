package workbench

import (
	"github.com/miroslav-matejovsky/inspector/model"
	"github.com/miroslav-matejovsky/inspector/source"
)

// InspectedModel builds the model of the inspected system from summaries,
// as the pages do. Only tests use it.
func InspectedModel(summaries []source.TargetSummary) *model.Model {
	return model.Build(summaries, inspectedInterpreter)
}
