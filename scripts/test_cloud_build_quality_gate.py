import copy
from pathlib import Path
import unittest
import yaml
from validate_cloud_build_transition import quality_gate_errors


class BuildQualityDependencyTests(unittest.TestCase):
    def setUp(self):
        self.config=yaml.safe_load(Path('cloudbuild.yaml').read_text())

    def test_real_configuration_has_quality_before_any_cloud_mutation(self):
        self.assertEqual(quality_gate_errors(self.config),[])

    def test_rejects_moving_quality_after_rollout_or_skipping_a_failed_gate(self):
        self.config['steps'][0],self.config['steps'][1]=self.config['steps'][1],self.config['steps'][0]
        self.assertTrue(quality_gate_errors(self.config))
        self.setUp()
        for update in [{'waitFor':['-']},{'allowFailure':True},{'allowExitCodes':[1]}]:
            config=copy.deepcopy(self.config);config['steps'][1].update(update)
            self.assertTrue(quality_gate_errors(config))

    def test_rejects_mutable_builder_and_unrelated_sha(self):
        for key,value in [('name','gcr.io/google.com/cloudsdktool/google-cloud-cli:stable'),('args',['--sha=main'])]:
            config=copy.deepcopy(self.config);config['steps'][0][key]=value
            self.assertTrue(quality_gate_errors(config))


if __name__=='__main__':unittest.main()
