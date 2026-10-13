import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { BehaviorSubject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { provideFrontendPorts } from '../../frontend.providers';
import { ExploreApi } from './infrastructure/explore-api';
import { SpaceDetailPage } from './space-detail-page';

describe('SpaceDetailPage public detail',()=>{
 afterEach(()=>TestBed.resetTestingModule());
 it('loads a minimized listing anonymously and shows a photo placeholder',async()=>{
  const params=new BehaviorSubject(convertToParamMap({id:'space-123'}));
  const api={publicDetail:vi.fn().mockResolvedValue({space_id:'space-123',category_code:'oficina',category_name:'Oficina',title:'Sala pública',description:'Descripción pública',area_m2:30,rate_unit:'dia',base_price_clp:'9007199254740993',currency:'CLP'})};
  TestBed.configureTestingModule({imports:[SpaceDetailPage],providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([]),{provide:ActivatedRoute,useValue:{paramMap:params.asObservable()}},{provide:ExploreApi,useValue:api}]});
  const fixture=TestBed.createComponent(SpaceDetailPage);fixture.detectChanges();await vi.waitFor(()=>expect(api.publicDetail).toHaveBeenCalledWith('space-123'));await vi.waitFor(()=>expect((fixture.componentInstance as unknown as {space:()=>unknown}).space()).toBeTruthy());fixture.detectChanges();
  expect(fixture.nativeElement.textContent).toContain('Sala pública');expect(fixture.nativeElement.textContent).toContain('CLP 9.007.199.254.740.993');expect(fixture.nativeElement.textContent).toContain('Fotografía no disponible');fixture.destroy();
 });
 it('keeps public detail during identity changes',async()=>{
  const params=new BehaviorSubject(convertToParamMap({id:'space-123'}));
  const api={publicDetail:vi.fn().mockResolvedValue({space_id:'space-123',category_code:'oficina',category_name:'Oficina',title:'Visible',description:'Descripción',area_m2:30,rate_unit:'dia',base_price_clp:'6000',currency:'CLP'})};
  TestBed.configureTestingModule({imports:[SpaceDetailPage],providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([]),{provide:ActivatedRoute,useValue:{paramMap:params.asObservable()}},{provide:ExploreApi,useValue:api}]});
  const fixture=TestBed.createComponent(SpaceDetailPage);fixture.detectChanges();await vi.waitFor(()=>expect(api.publicDetail).toHaveBeenCalled());await vi.waitFor(()=>expect((fixture.componentInstance as unknown as {space:()=>unknown}).space()).toBeTruthy());fixture.detectChanges();expect(fixture.nativeElement.textContent).toContain('Visible');fixture.destroy();
 });
 it('loads private availability only after a confirmed renter session',async()=>{
  const params=new BehaviorSubject(convertToParamMap({id:'space-123'}));
  const full={space_id:'space-123',category_code:'oficina',category_name:'Oficina',title:'Visible',description:'Descripción',area_m2:30,rate_unit:'hora',base_price_clp:'6000',currency:'CLP',time_zone:'America/Santiago',profile_version:1,profile:{},attributes:{}};
  const interval={start_at:'2026-10-14T12:00:00Z',end_at:'2026-10-14T13:00:00Z'};
  const api={publicDetail:vi.fn().mockResolvedValue({...full,time_zone:undefined,profile_version:undefined,profile:undefined,attributes:undefined}),detail:vi.fn().mockResolvedValue(full),profile:vi.fn().mockResolvedValue({category_code:'oficina',schema_version:1,attributes:[]}),businessClock:vi.fn().mockResolvedValue({data:{now:'2026-10-13T12:00:00Z'}}),availability:vi.fn().mockResolvedValue({data:{space_id:'space-123',time_zone:'America/Santiago',rate_unit:'hora',items:[interval]}}),quote:vi.fn().mockResolvedValue({safety_notice:'ENSAYO LOCAL — SIN COBRO REAL',data:{space_id:'space-123',start_at:interval.start_at,end_at:interval.end_at,subtotal_clp:'6000',unit_price_clp:'6000',guarantee_expected_clp:'50000'}})};
  TestBed.configureTestingModule({imports:[SpaceDetailPage],providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([]),PrivateSessionState,{provide:ActivatedRoute,useValue:{paramMap:params.asObservable()}},{provide:IdentityApi,useValue:{login:vi.fn().mockResolvedValue({account_id:'renter',roles:['arrendatario'],access_token:'private-bearer',token_type:'Bearer',expires_at:'2027-01-01T00:00:00Z',restricted_mode:false}),session:vi.fn().mockResolvedValue({account_id:'renter',roles:['arrendatario'],restricted_mode:false}),logout:vi.fn()},},{provide:ExploreApi,useValue:api}]});
  const fixture=TestBed.createComponent(SpaceDetailPage);fixture.detectChanges();await vi.waitFor(()=>expect(api.publicDetail).toHaveBeenCalledWith('space-123'));expect(api.detail).not.toHaveBeenCalled();
  await TestBed.inject(IdentitySession).signIn('renter@example.invalid','Synthetic#123');await vi.waitFor(()=>expect(api.availability).toHaveBeenCalled());
  expect(api.detail).toHaveBeenCalledWith('private-bearer','space-123');expect(fixture.nativeElement.textContent).toContain('Cotizar horario');
  const page=fixture.componentInstance as unknown as {chooseInterval(value:string):void;createQuote():Promise<void>;quote:()=>unknown};page.chooseInterval('0');await page.createQuote();
  expect(api.quote).toHaveBeenCalledWith('private-bearer','space-123',interval.start_at,interval.end_at);expect(page.quote()).toBeTruthy();fixture.destroy();
 });
});
