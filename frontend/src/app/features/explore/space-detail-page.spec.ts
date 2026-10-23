import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { BehaviorSubject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { BookingApiError, BookingPort } from '../bookings/domain/ports/booking-port';
import { provideFrontendPorts } from '../../frontend.providers';
import { ExploreApi } from './infrastructure/explore-api';
import { SpaceDetailPage } from './space-detail-page';
import type { BookingQuote } from './domain/ports/explore-port';

describe('SpaceDetailPage public detail',()=>{
 afterEach(()=>TestBed.resetTestingModule());
 it('loads a minimized listing anonymously and shows a photo placeholder',async()=>{
  const params=new BehaviorSubject(convertToParamMap({id:'space-123'}));
  const api={publicDetail:vi.fn().mockResolvedValue({space_id:'space-123',category_code:'oficina',category_name:'Oficina',title:'Sala pública',description:'Descripción pública',area_m2:30,rate_unit:'dia',base_price_clp:'9007199254740993',currency:'CLP',comuna_name:'Providencia',region_name:'Metropolitana',usage_rules:'No fumar',features:[{code:'wifi',label:'Wifi',value:true}]})};
  TestBed.configureTestingModule({imports:[SpaceDetailPage],providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([]),{provide:ActivatedRoute,useValue:{paramMap:params.asObservable()}},{provide:ExploreApi,useValue:api}]});
  const fixture=TestBed.createComponent(SpaceDetailPage);fixture.detectChanges();await vi.waitFor(()=>expect(api.publicDetail).toHaveBeenCalledWith('space-123'));await vi.waitFor(()=>expect((fixture.componentInstance as unknown as {space:()=>unknown}).space()).toBeTruthy());fixture.detectChanges();
  expect(fixture.nativeElement.textContent).toContain('Sala pública');expect(fixture.nativeElement.textContent).toContain('CLP 9.007.199.254.740.993');expect(fixture.nativeElement.textContent).toContain('Providencia, Metropolitana');expect(fixture.nativeElement.textContent).toContain('Fotografía no disponible');
  expect([...fixture.nativeElement.querySelectorAll('.detail-index a')].map((link:HTMLAnchorElement)=>link.getAttribute('href'))).toEqual(['#detail-overview','#detail-attributes','#detail-next-step']);
  expect(fixture.nativeElement.querySelector('#detail-attributes h2')?.textContent).toContain('Reglas de uso');fixture.destroy();
 });
 it('offers a retry when a public detail cannot be loaded',async()=>{
  const params=new BehaviorSubject(convertToParamMap({id:'space-123'}));
  const api={publicDetail:vi.fn().mockRejectedValueOnce(new Error('synthetic unavailable')).mockResolvedValue({space_id:'space-123',category_code:'oficina',category_name:'Oficina',title:'Recuperado',description:'Descripción pública',area_m2:30,rate_unit:'dia',base_price_clp:'9000',currency:'CLP'})};
  TestBed.configureTestingModule({imports:[SpaceDetailPage],providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([]),{provide:ActivatedRoute,useValue:{paramMap:params.asObservable()}},{provide:ExploreApi,useValue:api}]});
  const fixture=TestBed.createComponent(SpaceDetailPage);fixture.detectChanges();await vi.waitFor(()=>expect(api.publicDetail).toHaveBeenCalledOnce());await vi.waitFor(()=>expect(fixture.nativeElement.querySelector('.detail-load-error button')?.textContent).toContain('Volver a intentar'));
  (fixture.nativeElement.querySelector('.detail-load-error button') as HTMLButtonElement).click();fixture.detectChanges();await vi.waitFor(()=>expect(api.publicDetail).toHaveBeenCalledTimes(2));await vi.waitFor(()=>expect(fixture.nativeElement.textContent).toContain('Recuperado'));fixture.destroy();
 });
 it('renders every public reference with the cover first and omits images that return 404',async()=>{
  const params=new BehaviorSubject(convertToParamMap({id:'space-123'}));
  const api={publicDetail:vi.fn().mockResolvedValue({space_id:'space-123',category_code:'oficina',category_name:'Oficina',title:'Galería',description:'Descripción',area_m2:30,rate_unit:'dia',base_price_clp:'9000',currency:'CLP',images:[{image_id:'photo-a',is_cover:false},{image_id:'photo-cover',is_cover:true}]}),publicImageUrl:vi.fn((_spaceId:string,imageId:string)=>`/public/${imageId}`)};
  TestBed.configureTestingModule({imports:[SpaceDetailPage],providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([]),{provide:ActivatedRoute,useValue:{paramMap:params.asObservable()}},{provide:ExploreApi,useValue:api}]});
  const fixture=TestBed.createComponent(SpaceDetailPage);fixture.detectChanges();await vi.waitFor(()=>expect(api.publicDetail).toHaveBeenCalled());await vi.waitFor(()=>expect((fixture.componentInstance as unknown as {space:()=>unknown}).space()).toBeTruthy());fixture.detectChanges();
  const page=fixture.componentInstance as unknown as {orderedImages(images:{image_id:string;is_cover:boolean}[]):{image_id:string;is_cover:boolean}[];imageFailed(spaceId:string,imageId:string,generation:number):void;imageAvailable(spaceId:string,imageId:string):boolean;selectImage(spaceId:string,imageId:string):void;imageSelected(spaceId:string,imageId:string):boolean;spaceGeneration:()=>number};
  const images=[{image_id:'photo-a',is_cover:false},{image_id:'photo-cover',is_cover:true}];
  expect(page.orderedImages(images)).toEqual([{image_id:'photo-cover',is_cover:true},{image_id:'photo-a',is_cover:false}]);
  expect(fixture.nativeElement.querySelectorAll('.gallery-image img')).toHaveLength(2);
  expect(fixture.nativeElement.querySelectorAll('button.gallery-image')).toHaveLength(2);expect(fixture.nativeElement.querySelector('.gallery-feature img')?.getAttribute('src')).toBe('/public/photo-cover');page.selectImage('space-123','photo-a');fixture.detectChanges();expect(page.imageSelected('space-123','photo-a')).toBe(true);expect(fixture.nativeElement.querySelector('.gallery-feature img')?.getAttribute('src')).toBe('/public/photo-a');
  page.imageFailed('space-123','photo-a',page.spaceGeneration());fixture.detectChanges();expect(fixture.nativeElement.querySelectorAll('.gallery-image img')).toHaveLength(1);expect(fixture.nativeElement.querySelector('.gallery-feature img')?.getAttribute('src')).toBe('/public/photo-cover');
  page.imageFailed('space-old','photo-cover',page.spaceGeneration());expect(page.imageAvailable('space-123','photo-cover')).toBe(true);
  fixture.destroy();
 });
 it('ignores a late detail and image failure from the route that was left',async()=>{
  const params=new BehaviorSubject(convertToParamMap({id:'space-a'}));let resolveA!:(value:unknown)=>void;
  const api={publicDetail:vi.fn((id:string)=>id==='space-a'?new Promise(resolve=>{resolveA=resolve;}):Promise.resolve({space_id:'space-b',category_code:'oficina',category_name:'Oficina',title:'Espacio B',description:'Descripción',area_m2:20,rate_unit:'dia',base_price_clp:'9000',currency:'CLP',images:[{image_id:'photo-b',is_cover:true}]})),publicImageUrl:vi.fn((_spaceId:string,imageId:string)=>`/public/${imageId}`)};
  TestBed.configureTestingModule({imports:[SpaceDetailPage],providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([]),{provide:ActivatedRoute,useValue:{paramMap:params.asObservable()}},{provide:ExploreApi,useValue:api}]});
  const fixture=TestBed.createComponent(SpaceDetailPage);fixture.detectChanges();await vi.waitFor(()=>expect(api.publicDetail).toHaveBeenCalledWith('space-a'));
  params.next(convertToParamMap({id:'space-b'}));await vi.waitFor(()=>expect((fixture.componentInstance as unknown as {space:()=>{space_id:string}|null}).space()?.space_id).toBe('space-b'));
  const page=fixture.componentInstance as unknown as {imageFailed(spaceId:string,imageId:string,generation:number):void;imageAvailable(spaceId:string,imageId:string):boolean;spaceGeneration:()=>number};
  page.imageFailed('space-a','photo-b',0);resolveA!({space_id:'space-a',category_code:'oficina',category_name:'Oficina',title:'Espacio A',description:'Descripción',area_m2:20,rate_unit:'dia',base_price_clp:'9000',currency:'CLP',images:[]});await Promise.resolve();fixture.detectChanges();
  expect((fixture.componentInstance as unknown as {space:()=>{title:string}|null}).space()?.title).toBe('Espacio B');expect(page.imageAvailable('space-b','photo-b')).toBe(true);fixture.destroy();
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
  TestBed.configureTestingModule({imports:[SpaceDetailPage],providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([]),PrivateSessionState,{provide:ActivatedRoute,useValue:{paramMap:params.asObservable()}},{provide:IdentityApi,useValue:{login:vi.fn().mockResolvedValue({account_id:'renter',roles:['arrendatario'],access_token:'private-bearer',token_type:'Bearer',expires_at:'2027-01-01T00:00:00Z',capabilities: [], restricted_mode:false}),session:vi.fn().mockResolvedValue({account_id:'renter',roles:['arrendatario'],restricted_mode:false}),logout:vi.fn()},},{provide:ExploreApi,useValue:api}]});
  const fixture=TestBed.createComponent(SpaceDetailPage);fixture.detectChanges();await vi.waitFor(()=>expect(api.publicDetail).toHaveBeenCalledWith('space-123'));expect(api.detail).not.toHaveBeenCalled();
  await TestBed.inject(IdentitySession).signIn('renter@example.invalid','Synthetic#123');await vi.waitFor(()=>expect(api.availability).toHaveBeenCalled());
  expect(api.detail).toHaveBeenCalledWith('private-bearer','space-123');await vi.waitFor(()=>expect(fixture.nativeElement.textContent).toContain('Cotizar horario'));
  const page=fixture.componentInstance as unknown as {chooseInterval(value:string):void;createQuote():Promise<void>;quote:()=>unknown};page.chooseInterval('0');await page.createQuote();
  expect(api.quote).toHaveBeenCalledWith('private-bearer','space-123',interval.start_at,interval.end_at);expect(page.quote()).toBeTruthy();fixture.destroy();
 });

 it.each([
  { label: 'restricted', roles: ['arrendatario'], capabilities: [], restricted_mode: true, expected: 'modo restringido' },
  { label: 'role-incompatible', roles: ['arrendador'], capabilities: [], restricted_mode: false, expected: 'no tiene el rol de arrendatario' },
 ] as const)('keeps public detail and explains a $label session without private booking reads',async({roles,restricted_mode,expected})=>{
  const params=new BehaviorSubject(convertToParamMap({id:'space-123'}));
  const api={publicDetail:vi.fn().mockResolvedValue({space_id:'space-123',category_code:'oficina',category_name:'Oficina',title:'Visible',description:'Descripción',area_m2:30,rate_unit:'dia',base_price_clp:'6000',currency:'CLP'}),detail:vi.fn(),profile:vi.fn(),businessClock:vi.fn(),availability:vi.fn(),quote:vi.fn()};
  const identityApi={login:vi.fn().mockResolvedValue({account_id:'other-session',roles,access_token:'other-bearer',token_type:'Bearer',expires_at:'2027-01-01T00:00:00Z',restricted_mode}),session:vi.fn().mockResolvedValue({account_id:'other-session',roles,restricted_mode}),logout:vi.fn()};
  TestBed.configureTestingModule({imports:[SpaceDetailPage],providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([]),PrivateSessionState,{provide:ActivatedRoute,useValue:{paramMap:params.asObservable()}},{provide:IdentityApi,useValue:identityApi},{provide:ExploreApi,useValue:api}]});
  const fixture=TestBed.createComponent(SpaceDetailPage);fixture.detectChanges();await vi.waitFor(()=>expect(api.publicDetail).toHaveBeenCalledWith('space-123'));
  await TestBed.inject(IdentitySession).signIn('other@example.invalid','Synthetic#123');fixture.detectChanges();
  expect(fixture.nativeElement.textContent).toContain(expected);expect(api.detail).not.toHaveBeenCalled();expect(api.availability).not.toHaveBeenCalled();expect(api.quote).not.toHaveBeenCalled();fixture.destroy();
 });

 it('discards a late quote from the replaced account and requires a fresh selection',async()=>{
  const params=new BehaviorSubject(convertToParamMap({id:'space-123'}));
  const full={space_id:'space-123',category_code:'oficina',category_name:'Oficina',title:'Visible',description:'Descripción',area_m2:30,rate_unit:'hora',base_price_clp:'6000',currency:'CLP',time_zone:'America/Santiago',profile_version:1,profile:{},attributes:{}};
  const interval={start_at:'2026-10-14T12:00:00Z',end_at:'2026-10-14T13:00:00Z'};let resolveQuote!:(value:unknown)=>void;
  const pendingQuote=new Promise(resolve=>{resolveQuote=resolve;});
  const api={publicDetail:vi.fn().mockResolvedValue({...full,time_zone:undefined,profile_version:undefined,profile:undefined,attributes:undefined}),detail:vi.fn().mockResolvedValue(full),profile:vi.fn().mockResolvedValue({category_code:'oficina',schema_version:1,attributes:[]}),businessClock:vi.fn().mockResolvedValue({data:{now:'2026-10-13T12:00:00Z'}}),availability:vi.fn().mockResolvedValue({data:{space_id:'space-123',time_zone:'America/Santiago',rate_unit:'hora',items:[interval]}}),quote:vi.fn(()=>pendingQuote)};
  const identityApi={login:vi.fn().mockResolvedValueOnce({account_id:'renter-a',roles:['arrendatario'],access_token:'bearer-a',token_type:'Bearer',expires_at:'2027-01-01T00:00:00Z',capabilities: [], restricted_mode:false}).mockResolvedValueOnce({account_id:'renter-b',roles:['arrendatario'],access_token:'bearer-b',token_type:'Bearer',expires_at:'2027-01-01T00:00:00Z',restricted_mode:false}),session:vi.fn().mockImplementation((token:string)=>Promise.resolve({account_id:token==='bearer-a'?'renter-a':'renter-b',roles:['arrendatario'],restricted_mode:false})),logout:vi.fn().mockResolvedValue(undefined)};
  TestBed.configureTestingModule({imports:[SpaceDetailPage],providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([]),PrivateSessionState,{provide:ActivatedRoute,useValue:{paramMap:params.asObservable()}},{provide:IdentityApi,useValue:identityApi},{provide:ExploreApi,useValue:api}]});
  const fixture=TestBed.createComponent(SpaceDetailPage);fixture.detectChanges();const identity=TestBed.inject(IdentitySession);await vi.waitFor(()=>expect(api.publicDetail).toHaveBeenCalled());
  await identity.signIn('renter-a@example.invalid','Synthetic#123');await vi.waitFor(()=>expect(api.availability).toHaveBeenCalled());
  const page=fixture.componentInstance as unknown as {chooseInterval(value:string):void;createQuote():Promise<void>;quote:()=>unknown};page.chooseInterval('0');const oldQuote=page.createQuote();await vi.waitFor(()=>expect(api.quote).toHaveBeenCalledOnce());
  await identity.signOut();await identity.signIn('renter-b@example.invalid','Synthetic#456');await vi.waitFor(()=>expect(api.detail).toHaveBeenCalledWith('bearer-b','space-123'));
  resolveQuote({safety_notice:'ENSAYO LOCAL — SIN COBRO REAL',data:{id:'quote-old',space_id:'space-123',start_at:interval.start_at,end_at:interval.end_at}});await oldQuote;
  expect(page.quote()).toBeNull();expect(api.quote).toHaveBeenCalledOnce();fixture.destroy();
 });

 it.each([
  { oldOutcome: 'success', nextAccount: 'renter-a' },
  { oldOutcome: 'error', nextAccount: 'renter-a' },
  { oldOutcome: 'success', nextAccount: 'renter-b' },
  { oldOutcome: 'error', nextAccount: 'renter-b' },
 ] as const)('does not let a late $oldOutcome clear a newer reservation request for $nextAccount',async({oldOutcome,nextAccount})=>{
  const params=new BehaviorSubject(convertToParamMap({id:'space-123'}));
  const full={space_id:'space-123',category_code:'oficina',category_name:'Oficina',title:'Visible',description:'Descripción',area_m2:30,rate_unit:'hora',base_price_clp:'6000',currency:'CLP',time_zone:'America/Santiago',profile_version:1,profile:{},attributes:{}};
  const api={publicDetail:vi.fn().mockResolvedValue({...full,time_zone:undefined,profile_version:undefined,profile:undefined,attributes:undefined}),detail:vi.fn().mockResolvedValue(full),profile:vi.fn().mockResolvedValue({category_code:'oficina',schema_version:1,attributes:[]}),businessClock:vi.fn().mockResolvedValue({data:{now:'2026-10-13T12:00:00Z'}}),availability:vi.fn().mockResolvedValue({data:{space_id:'space-123',time_zone:'America/Santiago',rate_unit:'hora',items:[]}})};
  let resolveFirst!:(value:unknown)=>void;let rejectFirst!:(error:unknown)=>void;let resolveSecond!:(value:unknown)=>void;let rejectSecond!:(error:unknown)=>void;
  const firstRequest=new Promise((resolve,reject)=>{resolveFirst=resolve;rejectFirst=reject;});
  const secondRequest=new Promise((resolve,reject)=>{resolveSecond=resolve;rejectSecond=reject;});
  const booking={requestReservation:vi.fn().mockReturnValueOnce(firstRequest).mockReturnValueOnce(secondRequest)};
  const login=(account_id:string,access_token:string)=>({account_id,roles:['arrendatario'],access_token,token_type:'Bearer',expires_at:'2027-01-01T00:00:00Z',capabilities: [], restricted_mode:false});
  const confirmed=(account_id:string)=>({account_id,roles:['arrendatario'],capabilities: [], restricted_mode:false});
  TestBed.configureTestingModule({imports:[SpaceDetailPage],providers:[...provideFrontendPorts(),provideHttpClient(),provideRouter([]),PrivateSessionState,{provide:ActivatedRoute,useValue:{paramMap:params.asObservable()}},{provide:IdentityApi,useValue:{login:vi.fn().mockResolvedValueOnce(login('renter-a','bearer-a')).mockResolvedValueOnce(login(nextAccount,'bearer-b')),session:vi.fn().mockImplementation((token:string)=>Promise.resolve(confirmed(token==='bearer-a'?'renter-a':nextAccount))),logout:vi.fn().mockResolvedValue(undefined)}},{provide:ExploreApi,useValue:api},{provide:BookingPort,useValue:booking}]});
  const fixture=TestBed.createComponent(SpaceDetailPage);fixture.detectChanges();const identity=TestBed.inject(IdentitySession);
  await vi.waitFor(()=>expect(api.publicDetail).toHaveBeenCalledWith('space-123'));
  await identity.signIn('renter-a@example.invalid','Synthetic#123');await vi.waitFor(()=>expect(api.detail).toHaveBeenCalledWith('bearer-a','space-123'));
  const page=fixture.componentInstance as unknown as {quote:{set(value:BookingQuote|null):void};requestReservation():Promise<void>;requestingReservation():boolean};
  const setQuote=(id:string)=>page.quote.set({id,space_id:'space-123'} as BookingQuote);
  setQuote('quote-a');const pendingOld=page.requestReservation();await vi.waitFor(()=>expect(booking.requestReservation).toHaveBeenCalledTimes(1));expect(page.requestingReservation()).toBe(true);
  await identity.signOut();await identity.signIn(`${nextAccount}@example.invalid`,'Synthetic#456');
  await vi.waitFor(()=>expect(api.detail).toHaveBeenCalledWith('bearer-b','space-123'));
  expect(page.requestingReservation()).toBe(false);setQuote('quote-b');const pendingNew=page.requestReservation();await vi.waitFor(()=>expect(booking.requestReservation).toHaveBeenCalledTimes(2));expect(page.requestingReservation()).toBe(true);
  if(oldOutcome==='success')resolveFirst({safety_notice:'ENSAYO LOCAL — SIN COBRO REAL',data:{id:'old-reservation'}});else rejectFirst(new BookingApiError('old failure',503));
  await pendingOld;expect(page.requestingReservation()).toBe(true);
  rejectSecond(new BookingApiError('new failure',503));await pendingNew;expect(page.requestingReservation()).toBe(false);
  fixture.destroy();
 });
});
