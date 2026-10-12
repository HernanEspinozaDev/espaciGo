import { UiButton } from '../../shared/ui/button/button';
import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { categories } from './categories.data';

@Component({
  imports: [UiButton, RouterLink],
  selector: 'app-home',
  styleUrl: './home.scss',
  templateUrl: './home.html',
})
export class Home {
  protected readonly categories = categories;
}
