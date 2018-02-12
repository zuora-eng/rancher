package jenkins

import (
	//"encoding/xml"
	//"github.com/rancher/types/apis/management.cattle.io/v3"
	//"github.com/sirupsen/logrus"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"testing"
)

var pipeline = &v3.Pipeline{
	Spec: v3.PipelineSpec{
		Stages: []v3.Stage{{
			Name: "stage1",
			Steps: []v3.Step{
				{
					SourceCodeConfig: &v3.SourceCodeConfig{
						URL:    "https://github.com/gitlawr/php.git",
						Branch: "master",
						SourceCodeCredentialName: "user-ld9",
					},
				},
			},
		}, {
			Name: "stage2",
			Steps: []v3.Step{
				{
					RunScriptConfig: &v3.RunScriptConfig{
						Image:       "busybox",
						ShellScript: "echo hi",
					},
				},
			},
		},
		},
	},
}

func Test_convert(t *testing.T) {

	//pipelineJob := ConvertPipelineToJenkinsPipeline(&v3.Pipeline{})
	//bconf, _ := xml.MarshalIndent(pipelineJob, "  ", "    ")
	//logrus.Infof("got:\n%s", string(bconf))
}

func Test_Convert_Step(t *testing.T) {
	//step := &v3.Step{
	//	SourceCodeConfig: &v3.SourceCodeConfig{
	//		URL:    "https://github.com/gitlawr/php.git",
	//		Branch: "master",
	//		SourceCodeCredentialName: "user-ld9",
	//	},
	//}
	result := convertStep(pipeline, 1, 1)
	logrus.Infof(result)
}

func Test_Convert_Stage(t *testing.T) {
	//stage := &v3.Stage{
	//	Name: "stage-name",
	//	Steps: []v3.Step{
	//		{
	//			SourceCodeConfig: &v3.SourceCodeConfig{
	//				URL:    "https://github.com/gitlawr/php.git",
	//				Branch: "master",
	//				SourceCodeCredentialName: "user-ld9",
	//			},
	//		},
	//	},
	//}

	result := convertStage(pipeline, 1)
	logrus.Infof(result)
}

func Test_Convert_Pipeline(t *testing.T) {

	result := convertPipeline(pipeline)
	logrus.Infof(result)
}
