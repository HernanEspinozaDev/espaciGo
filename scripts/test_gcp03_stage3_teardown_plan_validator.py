import unittest
from gcp03_stage3_teardown_plan_validator import EXPECTED_ADDRESSES, EXPECTED_VARIABLES, PlanValidationError, REQUIRED_GRAPH_EDGES, validate_plan

def fixture():
    addresses=sorted(EXPECTED_ADDRESSES)
    return {
        'complete':True,'applyable':True,'resource_drift':[],
        'variables':{k:{'value':v} for k,v in EXPECTED_VARIABLES.items()},
        'resource_changes':[{'address':a,'mode':'managed','type':a.split('.')[0],'change':{'actions':['delete']}} for a in addresses],
        'prior_state':{'values':{'root_module':{'resources':[{'address':a,'type':a.split('.')[0]} for a in addresses]}}},
    }

def graph_fixture():
    return '\n'.join(f'"[root] {a} (destroy)" -> "[root] {b} (destroy)";' for a,b in REQUIRED_GRAPH_EDGES)

class Stage3TeardownPlanValidatorTests(unittest.TestCase):
    def test_accepts_exact_delete_plan_and_graph(self):
        result=validate_plan(fixture(),graph_fixture())
        self.assertEqual(result['summary'],{'add':0,'change':0,'delete':30})
        self.assertFalse(result['applied'])
        self.assertEqual(result['graph_edges_checked'],len(REQUIRED_GRAPH_EDGES))
    def test_rejects_incomplete_plan(self):
        plan=fixture(); plan['complete']=False
        with self.assertRaises(PlanValidationError): validate_plan(plan)
    def test_rejects_create_and_update(self):
        for action in (['create'],['update']):
            with self.subTest(action=action):
                plan=fixture(); plan['resource_changes'][0]['change']['actions']=action
                with self.assertRaises(PlanValidationError): validate_plan(plan)
    def test_rejects_missing_inventory(self):
        plan=fixture(); plan['resource_changes'].pop()
        with self.assertRaises(PlanValidationError): validate_plan(plan)
    def test_rejects_wrong_gate(self):
        plan=fixture(); plan['variables']['enable_restore_jobs']['value']=True
        with self.assertRaises(PlanValidationError): validate_plan(plan)
    def test_rejects_drift(self):
        plan=fixture(); plan['resource_drift']=[{'address':'google_sql_database.restore[0]'}]
        with self.assertRaises(PlanValidationError): validate_plan(plan)
    def test_rejects_sql_user(self):
        plan=fixture(); plan['prior_state']['values']['root_module']['resources'].append({'address':'google_sql_user.unexpected[0]','type':'google_sql_user'})
        with self.assertRaises(PlanValidationError): validate_plan(plan)
    def test_rejects_missing_dependency(self):
        with self.assertRaises(PlanValidationError): validate_plan(fixture(),graph_fixture().replace(REQUIRED_GRAPH_EDGES[0][0],'missing_dependency',1))

if __name__=='__main__': unittest.main()
